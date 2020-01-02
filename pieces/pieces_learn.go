package main

import (
	"fmt"
)

func main() {
	var s []int
	PrintPiecesInfo(s)
	s = append(s, 0)
	PrintPiecesInfo(s)
	s = append(s, 1)
	PrintPiecesInfo(s)
	s = append(s, 2, 3, 4, 5, 6, 7, 8)
	PrintPiecesInfo(s)
	for k, v := range s {
		PrintPiecesInfo([]int{k, v})
	}
}

// PrintPiecesInfo print pieces info includes length capacity and value.
func PrintPiecesInfo(s []int) {
	fmt.Printf("len = %d , cap = %d , %v\n", len(s), cap(s), s)
}
