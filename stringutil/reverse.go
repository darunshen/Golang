// Package stringutil contains utility functions for working with strings.
package stringutil

import (
	"strings"
)

// Reverse returns its argument string reversed rune-wise left to right.
func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

//StringCustom a custom data type
type StringCustom struct {
	Data string
}

//ToUpper to upper function for StringCustom
func (sc StringCustom) ToUpper() string {
	sc.Data = strings.ToUpper(sc.Data)
	return sc.Data
}
