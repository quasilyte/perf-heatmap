package heatmap

import (
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
