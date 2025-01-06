package v1

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
)

type LiquidationUsersResp struct {
	Users []User `json:"users"`
}

type User struct {
	UserAddress          common.Address  `json:"userAddress"`
	TokenName            string          `json:"tokenName"`
	Collateral           common.Address  `json:"collateral"`
	LiquidationCost      decimal.Decimal `json:"liquidationCost"`
	RangeFromLiquidation decimal.Decimal `json:"rangeFromLiquidation"`
	LiquidationPrice     decimal.Decimal `json:"liquidationPrice"`
}
