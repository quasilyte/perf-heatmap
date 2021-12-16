package heatmap

import (
	"fmt"
)

func forChunks(length, n int, visit func(chunkNum, i int)) {
	if length == 0 {
		return
	}

	// Using Bresenham's line algorithm to create chunks.
	// https://en.wikipedia.org/wiki/Bresenham%27s_line_algorithm

	i := 0
	acc := 0
	prev := 0
	chunkNum := 0
	for prev < length {
		acc += length
		chunkSize := acc / n
		if chunkSize > 0 {
			for j := 0; j < chunkSize; j++ {
				visit(chunkNum, i)
				i++
			}
			chunkNum++
			prev += chunkSize
			acc %= n
		}
	}

	if length >= n {
		if chunkNum != n {
			panic(fmt.Sprintf("[length=%d, n=%d] chunkNum=%d", length, n, chunkNum))
		}
	}
}
