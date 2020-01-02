package main

import (
	"fmt"

	stringutil "github.com/darunshen/go/stringutil"
)

func main() {
	tmp := stringutil.StringCustom{Data: "!oG ,olleH"}
	fmt.Println(stringutil.Reverse(tmp.ToUpper()))
	fmt.Println(stringutil.Reverse(tmp.Data))
}
