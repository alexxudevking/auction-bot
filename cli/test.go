package cli

import "fmt"

const (
	testAction  = "test"
	test2Action = "test2"
	test3Action = "test3"
	test4Action = "test4"
	test5Action = "test5"
)

func test(args []string) bool {
	if len(args) == 0 {
		fmt.Println("input something.....")

		panic(fmt.Sprintf("plnease set bod mode %v", []string{
			testAction,
		}))
	}

	return true
}
