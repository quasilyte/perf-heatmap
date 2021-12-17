package heatmap

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/pprof/profile"
)

func BenchmarkQuery(b *testing.B) {
	benchQueryLineRange := func(b *testing.B, suite *benchIndex, filename string, fromLine, toLine int, hit bool) {
		suffix := "hit"
		if !hit {
			suffix = "miss"
		}
		key := fmt.Sprintf("QueryLineRange/%s/%s_%dto%d_%s", suite.name, filepath.Base(filename), fromLine, toLine, suffix)
		b.Run(key, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				levels := 0
				suite.i.QueryLineRange(filename, fromLine, toLine, func(level HeatLevel) bool {
					levels += level.Local + level.Global
					return true
				})
				if hit {
					if levels == 0 {
						b.Fatal("expected a hit, got a miss")
					}
				} else {
					if levels != 0 {
						b.Fatal("expected a miss, got a hit")
					}
				}
			}
		})
	}

	benchQueryLine := func(b *testing.B, suite *benchIndex, filename string, line int, hit bool) {
		suffix := "hit"
		if !hit {
			suffix = "miss"
		}
		key := fmt.Sprintf("QueryLine/%s/%s_%d_%s", suite.name, filepath.Base(filename), line, suffix)
		b.Run(key, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				result := suite.i.QueryLine(filename, line)
				if hit {
					if result.Local+result.Global == 0 {
						b.Fatal("expected a hit, got a miss")
					}
				} else {
					if result.Local+result.Global != 0 {
						b.Fatal("expected a miss, got a hit")
					}
				}
			}
		})
	}

	benchQueryLineRange(b, benchIndexList[1], "/home/user/proj/data/matrix.go", 110, 150, false)
	benchQueryLineRange(b, benchIndexList[1], "/home/user/proj/data/matrix.go", 195, 202, true)
	benchQueryLineRange(b, benchIndexList[1], "/home/user/proj/data/matrix.go", 200, 1050, true)

	benchQueryLine(b, benchIndexList[0], "/home/user/proj/badfile.go", 50, false)
	benchQueryLine(b, benchIndexList[0], "/home/user/proj/data/data.go", 67, false)
	benchQueryLine(b, benchIndexList[0], "/home/user/proj/data/data.go", 70, true)
	benchQueryLine(b, benchIndexList[1], "/home/user/proj/data/matrix.go", 150, false)
	benchQueryLine(b, benchIndexList[1], "/home/user/proj/data/matrix.go", 100, true)
	benchQueryLine(b, benchIndexList[1], "/home/user/proj/data/matrix.go", 201, true)
}

type benchIndex struct {
	name string
	i    *Index
}

func benchIndexFromProfile(name string, config IndexConfig, p *profile.Profile) *benchIndex {
	index := NewIndex(config)
	if err := index.AddProfile(p); err != nil {
		panic(err)
	}
	return &benchIndex{name: name, i: index}
}

var benchIndexList = []*benchIndex{
	benchIndexFromProfile(
		"small",
		IndexConfig{Threshold: 1.0},
		newTestProfileBuilder().
			AddSamples("/home/user/proj/foo.go/example",
				15, []int{5, 19, 25, 90},
				26, []int{5, 19, 25, 40},
				30, []int{5, 20},
				10, []int{5, 19}).
			AddSamples("/home/user/proj/data/data.go/mul",
				60, []int{70},
				56, []int{70},
				30, []int{70, 75},
				67, []int{70},
				100, []int{60, 50, 109}).
			AddSamples("/home/user/go/src/runtime/slice.go/growslice",
				30, []int{921},
				54, []int{921},
				40, []int{921},
				20, []int{921},
				54, []int{921},
				61, []int{921, 1029},
				94, []int{194}).
			Build(),
	),

	benchIndexFromProfile(
		"average",
		IndexConfig{Threshold: 1.0},
		newTestProfileBuilder().
			AddSamples("/home/user/proj/foo.go/example",
				15, []int{5, 19, 25, 90},
				30, []int{5, 20},
				59, []int{5, 19, 25, 40},
				10, []int{5, 19},
				26, []int{5, 19, 25, 40},
				100, []int{5, 90, 96, 48, 93},
				30, []int{5, 20},
				10, []int{5, 19}).
			AddSamples("/home/user/proj/data/data.go/div",
				60, []int{70},
				56, []int{70},
				56, []int{70},
				56, []int{70},
				56, []int{70},
				30, []int{70, 75},
				67, []int{70},
				100, []int{60, 50, 109}).
			AddSamples("/home/user/proj/data/matrix.go/newMatrix",
				100, []int{207},
				100, []int{500, 305},
				100, []int{207},
				100, []int{200, 205, 201},
				100, []int{205},
				100, []int{100},
				100, []int{100, 200},
				10, []int{207},
				10, []int{500, 305},
				10, []int{207},
				10, []int{200, 205, 201},
				10, []int{205},
				10, []int{100},
				10, []int{100, 200},
				92, []int{207},
				92, []int{500, 305},
				92, []int{207},
				92, []int{200, 205, 201},
				92, []int{205},
				92, []int{100},
				92, []int{100, 200},
				49, []int{207},
				49, []int{500, 305},
				49, []int{207},
				49, []int{200, 205, 201},
				49, []int{205},
				49, []int{100},
				49, []int{100, 200},
				24, []int{207},
				24, []int{500, 305},
				24, []int{207},
				24, []int{200, 205, 201},
				24, []int{205},
				24, []int{100},
				24, []int{100, 200},
				30, []int{207},
				30, []int{500, 305},
				30, []int{207},
				30, []int{200, 205, 201},
				30, []int{205},
				30, []int{100},
				30, []int{100, 200},
				15, []int{207},
				15, []int{500, 305},
				15, []int{207},
				15, []int{200, 205, 201},
				15, []int{205},
				15, []int{100},
				15, []int{100, 200},
				15, []int{100, 200},
				15, []int{100},
				15, []int{205},
				15, []int{200, 205, 201},
				15, []int{207},
				15, []int{500, 305},
				15, []int{207},
				30, []int{100, 200},
				30, []int{100},
				30, []int{205},
				30, []int{200, 205, 201},
				30, []int{207},
				30, []int{500, 305},
				30, []int{207},
				24, []int{100, 200},
				24, []int{100},
				24, []int{205},
				24, []int{200, 205, 201},
				24, []int{207},
				24, []int{500, 305},
				24, []int{207},
				49, []int{100, 200},
				49, []int{100},
				49, []int{205},
				49, []int{200, 205, 201},
				49, []int{207},
				49, []int{500, 305},
				49, []int{207},
				92, []int{100, 200},
				92, []int{100},
				92, []int{205},
				92, []int{200, 205, 201},
				92, []int{207},
				92, []int{500, 305},
				92, []int{207},
				10, []int{100, 200},
				10, []int{100},
				10, []int{205},
				10, []int{200, 205, 201},
				10, []int{207},
				10, []int{500, 305},
				10, []int{207},
				100, []int{100, 200},
				100, []int{100},
				100, []int{205},
				100, []int{200, 205, 201},
				100, []int{207},
				100, []int{500, 305},
				100, []int{207}).
			AddSamples("/home/user/go/src/runtime/slice.go/mallocgc",
				30, []int{921},
				54, []int{921},
				40, []int{921},
				20, []int{921},
				54, []int{730},
				61, []int{921, 1029},
				94, []int{194},
				30, []int{730},
				30, []int{730},
				54, []int{730},
				30, []int{730},
				30, []int{921},
				10, []int{921},
				10, []int{921, 432, 400, 182},
				58, []int{921, 432, 400, 182},
				61, []int{921, 1029},
				94, []int{194},
				54, []int{921},
				40, []int{921},
				20, []int{921},
				54, []int{921},
				61, []int{921, 1029},
				94, []int{194}).
			Build(),
	),
}
