package heatmap

import (
	"reflect"
	"testing"
)

func TestForChunks(t *testing.T) {
	tests := []struct {
		points []dataPoint
		n      int
		want   []int
	}{
		{
			make([]dataPoint, 7),
			5,
			[]int{1, 1, 2, 1, 2},
		},

		{
			make([]dataPoint, 0),
			5,
			[]int{},
		},
		{
			make([]dataPoint, 0),
			0,
			[]int{},
		},

		{
			make([]dataPoint, 0),
			2,
			[]int{},
		},
		{
			make([]dataPoint, 3),
			5,
			[]int{1, 1, 1},
		},
		{
			make([]dataPoint, 4),
			5,
			[]int{1, 1, 1, 1},
		},

		{
			make([]dataPoint, 1),
			1,
			[]int{1},
		},
		{
			make([]dataPoint, 3),
			1,
			[]int{3},
		},
		{
			make([]dataPoint, 3),
			2,
			[]int{1, 2},
		},

		{
			make([]dataPoint, 10),
			5,
			[]int{2, 2, 2, 2, 2},
		},
		{
			make([]dataPoint, 9),
			5,
			[]int{1, 2, 2, 2, 2},
		},
		{
			make([]dataPoint, 11),
			5,
			[]int{2, 2, 2, 2, 3},
		},
		{
			make([]dataPoint, 12),
			5,
			[]int{2, 2, 3, 2, 3},
		},
		{
			make([]dataPoint, 13),
			5,
			[]int{2, 3, 2, 3, 3},
		},

		{
			make([]dataPoint, 8),
			5,
			[]int{1, 2, 1, 2, 2},
		},
	}

	for _, test := range tests {
		have := []int{}
		currentChunk := -1
		forChunks(len(test.points), test.n, func(chunkNum, i int) {
			if chunkNum != currentChunk {
				have = append(have, 0)
				currentChunk = chunkNum
			}
			have[len(have)-1]++
		})
		if !reflect.DeepEqual(have, test.want) {
			t.Errorf("chunks(numPoints=%d, n=%d):\nhave: %#v\nwant: %#v", len(test.points), test.n, have, test.want)
		}
	}
}
