package jobs

import (
	"context"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/helio-money/auctionbot/internal/dao/v2/clipper"
	"github.com/helio-money/auctionbot/internal/dao/v2/interaction"
	"github.com/helio-money/auctionbot/internal/wallet"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"math/big"
	"sync"
	"time"
)

func NewResetAuctionJob(
	ctx context.Context,
	log *logrus.Logger,
	wall wallet.Walleter,
	ethCli *ethclient.Client,
	interactAddr common.Address,
	collateralAddr common.Address,
	withWait bool,
) Job {
	job := &resetJob{
		ctx:            ctx,
		ethCli:         ethCli,
		interactAddr:   interactAddr,
		collateralAddr: collateralAddr,
		wallet:         wall,
		log: log.WithFields(logrus.Fields{
			"job":      "reset_auction",
			"operator": wall.Address(),
		}),
		withWait: withWait,
	}

	return job
}

var _ Job = (*resetJob)(nil)

type resetJob struct {
	ctx context.Context

	wallet         wallet.Walleter
	ethCli         *ethclient.Client
	log            *logrus.Entry
	collateralAddr common.Address
	interactAddr   common.Address

	inter   *interaction.Interaction
	clipper *clipper.Clipper

	withWait bool
}

func (j *resetJob) init() {
	if j == nil {
		panic("buy auction job is null")
	}
	var err error

	j.inter, err = interaction.NewInteraction(j.interactAddr, j.ethCli)
	if err != nil {
		panic(err)
	}

	// get collateral details to retrieve clip and ilk
	collatDetails, err := j.inter.Collaterals(&bind.CallOpts{}, j.collateralAddr)
	if err != nil {
		panic(err)
	}

	j.clipper, err = clipper.NewClipper(collatDetails.Clip, j.ethCli)
	if err != nil {
		panic(err)
	}
}

func (j *resetJob) Run(ctx context.Context, wg *sync.WaitGroup) {
	j.init()

	ticker := time.NewTicker(time.Minute)
	go func() {
		j.log.Debug("start")

		defer wg.Done()
		for {
			select {
			case <-ticker.C:
				auctionIds, err := j.clipper.List(&bind.CallOpts{})
				if err != nil {
					j.log.WithError(err).Error("failed to list auction ids from clipper")
					continue
				}

				if len(auctionIds) == 0 {
					j.log.Debug("nothing to reset")
					continue
				}

				for _, auctionID := range auctionIds {
					j.processAuction(auctionID)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (j *resetJob) processAuction(auctionID *big.Int) {
	log := j.log.WithFields(logrus.Fields{
		"auction_id": auctionID.String(),
	})

	status, err := j.clipper.GetStatus(&bind.CallOpts{}, auctionID)
	if err != nil {
		log.WithError(err).Error("failed to get status from clipper")
		return
	}

	log = log.WithFields(logrus.Fields{
		"needs_redo": status.NeedsRedo,
		"lot":        status.Lot.String(),
		"liq_price":  status.Price,
	})

	if status.NeedsRedo {
		log.Debug("auction resetting...")
		if err := j.redoAuction(auctionID); err != nil {
			log.WithError(err).Error("failed to redo auction")
		}
		log.Debug("auction reset")
	}
}

func (j *resetJob) redoAuction(auctionID *big.Int) error {
	opts, err := j.wallet.Opts(j.ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tx opts")
	}

	tx, err := j.inter.ResetAuction(
		opts,
		j.collateralAddr,
		auctionID,
		opts.From,
	)
	if err != nil {
		return errors.Wrap(err, "failed to send tx")
	}

	if j.withWait {
		receipt, err := bind.WaitMined(j.ctx, j.ethCli, tx)
		if err != nil {
			return errors.Wrapf(err, "failed to wait for tx %s mint", tx.Hash())
		}

		if receipt.Status == types.ReceiptStatusFailed {
			_, err := getRevertReason(j.ctx, j.ethCli, tx, receipt)
			return errors.Wrapf(err, "failed to wait for tx %s mint", tx.Hash())
		}
	}

	return nil
}
