package heatmap

import (
	"fmt"
	"testing"

	"github.com/google/pprof/profile"
)

func BenchmarkQuery(b *testing.B) {
	benchQueryLineRange := func(b *testing.B, suite *benchIndex, keyString string, fromLine, toLine int, hit bool) {
		suffix := "hit"
		if !hit {
			suffix = "miss"
		}
		key := convertTestKey(keyString)
		benchName := fmt.Sprintf("QueryLine/%s/%s_%dto%d_%s", suite.name, key.Filename, fromLine, toLine, suffix)
		b.Run(benchName, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				levels := 0
				suite.i.QueryLineRange(key, fromLine, toLine, func(l LineStats) bool {
					levels += l.HeatLevel + l.GlobalHeatLevel
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

	benchQueryLine := func(b *testing.B, suite *benchIndex, keyString string, line int, hit bool) {
		suffix := "hit"
		if !hit {
			suffix = "miss"
		}
		key := convertTestKey(keyString)
		benchName := fmt.Sprintf("QueryLine/%s/%s_%d_%s", suite.name, key.Filename, line, suffix)
		b.Run(benchName, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				result := suite.i.QueryLine(key, line)
				if hit {
					if result.HeatLevel+result.GlobalHeatLevel == 0 {
						b.Fatal("expected a hit, got a miss")
					}
				} else {
					if result.HeatLevel+result.GlobalHeatLevel != 0 {
						b.Fatal("expected a miss, got a hit")
					}
				}
			}
		})
	}

	benchQueryLineRange(b, benchIndexList[1], "matrix.go:data.newMatrix", 110, 150, false)
	benchQueryLineRange(b, benchIndexList[1], "matrix.go:data.newMatrix", 195, 202, true)
	benchQueryLineRange(b, benchIndexList[1], "matrix.go:data.newMatrix", 200, 1050, true)

	benchQueryLine(b, benchIndexList[0], "badfile.go:foo.badpkg", 50, false)
	benchQueryLine(b, benchIndexList[0], "data.go:data.mul", 67, false)
	benchQueryLine(b, benchIndexList[0], "data.go:data.mul", 70, true)
	benchQueryLine(b, benchIndexList[1], "matrix.go:data.newMatrix", 150, false)
	benchQueryLine(b, benchIndexList[1], "matrix.go:data.newMatrix", 100, true)
	benchQueryLine(b, benchIndexList[1], "matrix.go:data.newMatrix", 201, true)
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
			AddSamples("/home/user/proj/foo.go:main.example",
				15000, []int{5, 19, 25, 90},
				26000, []int{5, 19, 25, 40},
				30000, []int{5, 20},
				10000, []int{5, 19}).
			AddSamples("/home/user/proj/data/data.go:data.mul",
				60000, []int{70},
				56000, []int{70},
				30000, []int{70, 75},
				67000, []int{70},
				100000, []int{60, 50, 109}).
			AddSamples("/home/user/go/src/runtime/slice.go:runtime.growslice",
				30000, []int{921},
				54000, []int{921},
				40000, []int{921},
				20000, []int{921},
				54000, []int{921},
				61000, []int{921, 1029},
				94000, []int{194}).
			Build(),
	),

	benchIndexFromProfile(
		"average",
		IndexConfig{Threshold: 1.0},
		newTestProfileBuilder().
			AddSamples("/home/user/proj/foo.go:main.example",
				15000, []int{5, 19, 25, 90},
				30000, []int{5, 20},
				59000, []int{5, 19, 25, 40},
				10000, []int{5, 19},
				26000, []int{5, 19, 25, 40},
				100000, []int{5, 90, 96, 48, 93},
				30000, []int{5, 20},
				10000, []int{5, 19}).
			AddSamples("/home/user/proj/data/data.go:data.div",
				60000, []int{70},
				56000, []int{70},
				56000, []int{70},
				56000, []int{70},
				56000, []int{70},
				30000, []int{70, 75},
				67000, []int{70},
				100000, []int{60, 50, 109}).
			AddSamples("/home/user/proj/data/matrix.go:data.newMatrix",
				100000, []int{207},
				100000, []int{500, 305},
				100000, []int{207},
				100000, []int{200, 205, 201},
				100000, []int{205},
				100000, []int{100},
				100000, []int{100, 200},
				10000, []int{207},
				10000, []int{500, 305},
				10000, []int{207},
				10000, []int{200, 205, 201},
				10000, []int{205},
				10000, []int{100},
				10000, []int{100, 200},
				92000, []int{207},
				92000, []int{500, 305},
				92000, []int{207},
				92000, []int{200, 205, 201},
				92000, []int{205},
				92000, []int{100},
				92000, []int{100, 200},
				49000, []int{207},
				49000, []int{500, 305},
				49000, []int{207},
				49000, []int{200, 205, 201},
				49000, []int{205},
				49000, []int{100},
				49000, []int{100, 200},
				24000, []int{207},
				24000, []int{500, 305},
				24000, []int{207},
				24000, []int{200, 205, 201},
				24000, []int{205},
				24000, []int{100},
				24000, []int{100, 200},
				30000, []int{207},
				30000, []int{500, 305},
				30000, []int{207},
				30000, []int{200, 205, 201},
				30000, []int{205},
				30000, []int{100},
				30000, []int{100, 200},
				15000, []int{207},
				15000, []int{500, 305},
				15000, []int{207},
				15000, []int{200, 205, 201},
				15000, []int{205},
				15000, []int{100},
				15000, []int{100, 200},
				15000, []int{100, 200},
				15000, []int{100},
				15000, []int{205},
				15000, []int{200, 205, 201},
				15000, []int{207},
				15000, []int{500, 305},
				15000, []int{207},
				30000, []int{100, 200},
				30000, []int{100},
				30000, []int{205},
				30000, []int{200, 205, 201},
				30000, []int{207},
				30000, []int{500, 305},
				30000, []int{207},
				24000, []int{100, 200},
				24000, []int{100},
				24000, []int{205},
				24000, []int{200, 205, 201},
				24000, []int{207},
				24000, []int{500, 305},
				24000, []int{207},
				49000, []int{100, 200},
				49000, []int{100},
				49000, []int{205},
				49000, []int{200, 205, 201},
				49000, []int{207},
				49000, []int{500, 305},
				49000, []int{207},
				92000, []int{100, 200},
				92000, []int{100},
				92000, []int{205},
				92000, []int{200, 205, 201},
				92000, []int{207},
				92000, []int{500, 305},
				92000, []int{207},
				10000, []int{100, 200},
				10000, []int{100},
				10000, []int{205},
				10000, []int{200, 205, 201},
				10000, []int{207},
				10000, []int{500, 305},
				10000, []int{207},
				100000, []int{100, 200},
				100000, []int{100},
				100000, []int{205},
				100000, []int{200, 205, 201},
				100000, []int{207},
				100000, []int{500, 305},
				100000, []int{207}).
			AddSamples("/home/user/go/src/runtime/slice.go:runtime.mallocgc",
				30000, []int{921},
				54000, []int{921},
				40000, []int{921},
				20000, []int{921},
				54000, []int{730},
				61000, []int{921, 1029},
				94000, []int{194},
				30000, []int{730},
				30000, []int{730},
				54000, []int{730},
				30000, []int{730},
				30000, []int{921},
				10000, []int{921},
				10000, []int{921, 432, 400, 182},
				58000, []int{921, 432, 400, 182},
				61000, []int{921, 1029},
				94000, []int{194},
				54000, []int{921},
				40000, []int{921},
				20000, []int{921},
				54000, []int{921},
				61000, []int{921, 1029},
				94000, []int{194}).
			Build(),
	),
}
