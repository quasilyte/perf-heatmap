package heatmap

import (
	"reflect"
	"testing"
)

func TestDataPointFlagsLevel(t *testing.T) {
	tests := []int{0, 1, 3, maxHeatLevel}
	for _, test := range tests {
		var f dataPointFlags
		if f.GetLocalLevel() != 0 {
			t.Fatalf("non-zero local level at the beginning")
		}
		if f.GetGlobalLevel() != 0 {
			t.Fatalf("non-zero global level at the beginning")
		}
		f.SetLocalLevel(test)
		have := f.GetLocalLevel()
		if have != test {
			t.Fatalf("local level mismatch after setting it to %d (got %d)", test, have)
		}
		if f.GetGlobalLevel() != 0 {
			t.Fatalf("unexpected non-zero global level")
		}
		f.SetGlobalLevel(test)
		if f.GetLocalLevel() != f.GetGlobalLevel() {
			t.Fatal("mismatching local and global levels")
		}
		f.SetLocalLevel(0)
		have = f.GetLocalLevel()
		if have != 0 {
			t.Fatalf("local level mismatch after setting it to 0 (got %d)", have)
		}
		have = f.GetGlobalLevel()
		if have != test {
			t.Fatalf("global level mismatch after setting it to %d (got %d)", test, have)
		}
		f.SetGlobalLevel(0)
		if f.GetGlobalLevel() != 0 {
			t.Fatalf("unexpected non-zero global level")
		}
	}

	for i := 0; i <= maxHeatLevel; i++ {
		for j := 0; j <= maxHeatLevel; j++ {
			var f dataPointFlags
			for repeats := 0; repeats < 3; repeats++ {
				f.SetLocalLevel(i)
				f.SetGlobalLevel(j)
				if f.GetLocalLevel() != i {
					t.Fatalf("[%d, %d] => local level mismatches", i, j)
				}
				if f.GetGlobalLevel() != j {
					t.Fatalf("[%d, %d] => global level mismatches", i, j)
				}
			}
		}
	}
}

func TestForChunks(t *testing.T) {
	tests := []struct {
		points []dataPoint
		n      int
		want   []int
	}{
		{
			make([]dataPoint, 7),
			5,
			[]int{3, 1, 1, 1, 1},
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
			[]int{3, 2, 2, 2, 2},
		},
		{
			make([]dataPoint, 12),
			5,
			[]int{4, 2, 2, 2, 2},
		},
		{
			make([]dataPoint, 13),
			5,
			[]int{1, 3, 3, 3, 3},
		},

		{
			make([]dataPoint, 8),
			5,
			[]int{2, 2, 2, 2},
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
