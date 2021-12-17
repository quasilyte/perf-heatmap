package heatmap

import (
	"fmt"
	"math/rand"
	"path"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/pprof/profile"
)

func TestAddProfile(t *testing.T) {
	dumpIndex := func(index *Index) []string {
		var lines []string
		for _, filename := range index.CollectFilenames() {
			currentFunc := ""
			index.InspectFileLines(filename, func(s LineStats) {
				if currentFunc != s.Func.Name {
					currentFunc = s.Func.Name
					lines = append(lines, fmt.Sprintf("func %s (L=%d G=%d)",
						currentFunc, s.Func.MaxHeatLevel, s.Func.MaxGlobalHeatLevel))
				}
				l := fmt.Sprintf("%s:%d: V=%3d L=%d G=%d", filename, s.LineNum, s.Value, s.HeatLevel, s.GlobalHeatLevel)
				lines = append(lines, l)
			})
		}
		return lines
	}

	type testQuery struct {
		filename string
		line     int
		want     HeatLevel
	}

	type testRangeQuery struct {
		filename string
		fromLine int
		toLine   int
		want     []HeatLevel
	}

	type testCase struct {
		buildProfile func() *profile.Profile
		config       IndexConfig
		want         []string
		queries      []testQuery
		rangeQueries []testRangeQuery
	}

	tests := []testCase{
		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go/example", 25, []int{10}).
				AddSamples("buffer.go/example", 75, []int{10}).
				Build,
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"func example (L=5 G=5)",
				"buffer.go:10: V=100 L=5 G=5",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go/example",
					25, []int{10},
					75, []int{10}).
				Build,
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"func example (L=5 G=5)",
				"buffer.go:10: V=100 L=5 G=5",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("/home/gopher/buffer.go/example",
					75, []int{10}).
				Build,
			config: IndexConfig{
				TrimPrefix: "/home/gopher/",
				Threshold:  0.25,
			},
			want: []string{
				"func example (L=5 G=5)",
				"buffer.go:10: V= 75 L=5 G=5",
			},
			queries: []testQuery{
				{"buffer.go", 10, HeatLevel{5, 5}},
				{"/home/gopher/buffer.go", 10, HeatLevel{}},
				{"/home/gopher/buffer.go", 9, HeatLevel{}},
				{"/home/gopher/buffer2.go", 10, HeatLevel{}},
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("/home/gopher/buffer.go/example",
					75, []int{10}).
				Build,
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"func example (L=5 G=5)",
				"/home/gopher/buffer.go:10: V= 75 L=5 G=5",
			},
			queries: []testQuery{
				{"/home/gopher/buffer.go", 10, HeatLevel{5, 5}},
				{"buffer.go", 10, HeatLevel{}},
				{"/home/gopher/buffer.go", 9, HeatLevel{}},
				{"/home/gopher/buffer2.go", 10, HeatLevel{}},
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go/example",
					75, []int{11, 12},
					25, []int{10}).
				Build,
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"func example (L=5 G=5)",
				"buffer.go:10: V= 25 L=0 G=0",
				"buffer.go:11: V= 75 L=0 G=0",
				"buffer.go:12: V= 75 L=5 G=5",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go/example",
					10, []int{5},
					11, []int{4},
					12, []int{3},
					13, []int{2},
					14, []int{1}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func example (L=5 G=5)",
				"buffer.go:1: V= 14 L=5 G=5",
				"buffer.go:2: V= 13 L=4 G=4",
				"buffer.go:3: V= 12 L=3 G=3",
				"buffer.go:4: V= 11 L=2 G=2",
				"buffer.go:5: V= 10 L=1 G=1",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go/example",
					10, []int{5},
					11, []int{4},
					12, []int{3},
					13, []int{2},
					14, []int{1}).
				Build,
			config: IndexConfig{Threshold: 0.6},
			want: []string{
				"func example (L=5 G=5)",
				"buffer.go:1: V= 14 L=5 G=5",
				"buffer.go:2: V= 13 L=4 G=4",
				"buffer.go:3: V= 12 L=3 G=3",
				"buffer.go:4: V= 11 L=0 G=0",
				"buffer.go:5: V= 10 L=0 G=0",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go/f2",
					100, []int{1, 2, 3},
					50, []int{2, 3},
					25, []int{3}).
				AddSamples("a.go/f1",
					150, []int{6},
					160, []int{6},
					80, []int{10},
					40, []int{11}).
				AddSamples("b.go/f",
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func f2 (L=4 G=3)",
				"a.go:1: V=100 L=2 G=2",
				"a.go:2: V=150 L=3 G=2",
				"a.go:3: V=175 L=4 G=3",

				"func f1 (L=5 G=5)",
				"a.go:6: V=310 L=5 G=5",
				"a.go:10: V= 80 L=1 G=1",
				"a.go:11: V= 40 L=1 G=1",

				"func f (L=5 G=4)",
				"b.go:5: V=200 L=4 G=4",
				"b.go:6: V=200 L=5 G=4",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go/f1",
					100, []int{1, 2, 3},
					50, []int{2, 3},
					25, []int{3},
					500, []int{4}).
				AddSamples("a.go/f2",
					150, []int{6},
					160, []int{6},
					80, []int{10},
					40, []int{11}).
				AddSamples("b.go/f",
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func f1 (L=5 G=5)",
				"a.go:1: V=100 L=2 G=2",
				"a.go:2: V=150 L=3 G=2",
				"a.go:3: V=175 L=3 G=3",
				"a.go:4: V=500 L=5 G=5",

				"func f2 (L=4 G=4)",
				"a.go:6: V=310 L=4 G=4",
				"a.go:10: V= 80 L=1 G=1",
				"a.go:11: V= 40 L=1 G=1",

				"func f (L=5 G=4)",
				"b.go:5: V=200 L=4 G=3",
				"b.go:6: V=200 L=5 G=4",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go/f1",
					100, []int{1, 2, 3},
					50, []int{2, 3},
					25, []int{3},
					500, []int{4}).
				AddSamples("a.go/f2",
					150, []int{6},
					160, []int{6},
					80, []int{10},
					40, []int{11},
					150, []int{14},
					160, []int{15},
					80, []int{16},
					40, []int{16},
					150, []int{17},
					160, []int{19},
					80, []int{24},
					40, []int{28}).
				AddSamples("b.go/f",
					40, []int{5, 6},
					40, []int{5, 7},
					40, []int{5, 7},
					40, []int{5, 6},
					40, []int{5, 6}).
				AddSamples("c.go/f",
					1, []int{1},
					2, []int{1},
					3, []int{1}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func f1 (L=5 G=5)",
				"a.go:1: V=100 L=2 G=2",
				"a.go:2: V=150 L=3 G=3",
				"a.go:3: V=175 L=4 G=4",
				"a.go:4: V=500 L=5 G=5",
				"func f2 (L=5 G=5)",
				"a.go:6: V=310 L=5 G=5",
				"a.go:10: V= 80 L=1 G=2",
				"a.go:11: V= 40 L=1 G=1",
				"a.go:14: V=150 L=3 G=3",
				"a.go:15: V=160 L=4 G=4",
				"a.go:16: V=120 L=2 G=3",
				"a.go:17: V=150 L=3 G=4",
				"a.go:19: V=160 L=4 G=4",
				"a.go:24: V= 80 L=2 G=2",
				"a.go:28: V= 40 L=1 G=1",
				"func f (L=5 G=5)",
				"b.go:5: V=200 L=5 G=5",
				"b.go:6: V=120 L=4 G=2",
				"b.go:7: V= 80 L=3 G=1",
				"func f (L=5 G=1)",
				"c.go:1: V=  6 L=5 G=1",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go/f1",
					100, []int{1, 2, 3},
					50, []int{2, 3},
					25, []int{3},
					500, []int{4}).
				AddSamples("a.go/f2",
					150, []int{6},
					200, []int{6},
					80, []int{10},
					40, []int{11}).
				AddSamples("b.go/f",
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{7},
					145, []int{7, 6, 5},
					40, []int{5, 6}).
				Build,
			config: IndexConfig{Threshold: 0.5},
			want: []string{
				"func f1 (L=5 G=5)",
				"a.go:1: V=100 L=0 G=0",
				"a.go:2: V=150 L=0 G=0",
				"a.go:3: V=175 L=3 G=0",
				"a.go:4: V=500 L=5 G=5",

				"func f2 (L=4 G=4)",
				"a.go:6: V=350 L=4 G=4",
				"a.go:10: V= 80 L=0 G=0",
				"a.go:11: V= 40 L=0 G=0",

				"func f (L=5 G=3)",
				"b.go:5: V=345 L=0 G=2",
				"b.go:6: V=345 L=5 G=3",
				"b.go:7: V=185 L=0 G=1",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go/f1", 109, []int{1}).
				AddSamples("a.go/f2", 108, []int{2}).
				AddSamples("a.go/f3", 107, []int{3}).
				AddSamples("a.go/f4", 106, []int{4}).
				AddSamples("a.go/f5", 105, []int{5}).
				AddSamples("a.go/f6", 104, []int{6}).
				AddSamples("a.go/f7", 103, []int{7}).
				AddSamples("a.go/f8", 102, []int{8}).
				AddSamples("a.go/f9", 101, []int{9}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func f1 (L=5 G=5)",
				"a.go:1: V=109 L=5 G=5",
				"func f2 (L=4 G=4)",
				"a.go:2: V=108 L=4 G=4",
				"func f3 (L=4 G=4)",
				"a.go:3: V=107 L=4 G=4",
				"func f4 (L=3 G=3)",
				"a.go:4: V=106 L=3 G=3",
				"func f5 (L=3 G=3)",
				"a.go:5: V=105 L=3 G=3",
				"func f6 (L=2 G=2)",
				"a.go:6: V=104 L=2 G=2",
				"func f7 (L=2 G=2)",
				"a.go:7: V=103 L=2 G=2",
				"func f8 (L=1 G=1)",
				"a.go:8: V=102 L=1 G=1",
				"func f9 (L=1 G=1)",
				"a.go:9: V=101 L=1 G=1",
			},
		},

		// All samples would point to the same line, resulting in a single data point.
		{
			buildProfile: newTestProfileBuilder().
				Sorted().
				AddSamples("a.go/f1", 109, []int{1}).
				AddSamples("a.go/f2", 108, []int{1}).
				AddSamples("a.go/f4", 106, []int{1}).
				AddSamples("a.go/f3", 107, []int{1}).
				AddSamples("a.go/f6", 104, []int{1}).
				AddSamples("a.go/f9", 101, []int{1}).
				AddSamples("a.go/f5", 105, []int{1}).
				AddSamples("a.go/f8", 102, []int{1}).
				AddSamples("a.go/f7", 103, []int{1}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func f1 (L=5 G=5)",
				"a.go:1: V=945 L=5 G=5",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go/f1", 109, []int{5}).
				AddSamples("a.go/f2", 108, []int{6}).
				AddSamples("a.go/f3", 107, []int{7}).
				AddSamples("a.go/f4", 106, []int{1}).
				AddSamples("a.go/f5", 105, []int{2}).
				AddSamples("a.go/f6", 104, []int{3}).
				AddSamples("a.go/f7", 103, []int{4}).
				AddSamples("a.go/f8", 102, []int{8}).
				AddSamples("a.go/f9", 101, []int{9}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func f4 (L=3 G=3)",
				"a.go:1: V=106 L=3 G=3",
				"func f5 (L=3 G=3)",
				"a.go:2: V=105 L=3 G=3",
				"func f6 (L=2 G=2)",
				"a.go:3: V=104 L=2 G=2",
				"func f7 (L=2 G=2)",
				"a.go:4: V=103 L=2 G=2",
				"func f1 (L=5 G=5)",
				"a.go:5: V=109 L=5 G=5",
				"func f2 (L=4 G=4)",
				"a.go:6: V=108 L=4 G=4",
				"func f3 (L=4 G=4)",
				"a.go:7: V=107 L=4 G=4",
				"func f8 (L=1 G=1)",
				"a.go:8: V=102 L=1 G=1",
				"func f9 (L=1 G=1)",
				"a.go:9: V=101 L=1 G=1",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("file.go/testfunc",
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
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func testfunc (L=5 G=5)",
				"file.go:100: V=1280 L=3 G=3",
				"file.go:200: V=1280 L=3 G=3",
				"file.go:201: V=640 L=1 G=1",
				"file.go:205: V=1280 L=4 G=4",
				"file.go:207: V=1280 L=5 G=5",
				"file.go:305: V=640 L=1 G=1",
				"file.go:500: V=640 L=2 G=2",
			},
			rangeQueries: []testRangeQuery{
				{
					filename: "file.go",
					fromLine: 110,
					toLine:   150,
					want:     []HeatLevel{},
				},
				// {
				// 	filename: "file.go",
				// 	fromLine: 195,
				// 	toLine:   205,
				// 	want: []HeatLevel{
				// 		{3, 3},
				// 		{1, 1},
				// 		{4, 4},
				// 	},
				// },
				// {
				// 	filename: "file.go",
				// 	fromLine: 200,
				// 	toLine:   205,
				// 	want: []HeatLevel{
				// 		{3, 3},
				// 		{1, 1},
				// 		{4, 4},
				// 	},
				// },
				// {
				// 	filename: "file.go",
				// 	fromLine: 202,
				// 	toLine:   205,
				// 	want: []HeatLevel{
				// 		{4, 4},
				// 	},
				// },
			},
		},
	}

	validateIndex := func(t *testing.T, index *Index) {
		for filename, f := range index.byFilename {
			if !index.HasFile(filename) {
				t.Fatalf("!HasFile(%s)", filename)

			}
			funcnames := make([]string, 0, len(f.funcs))
			for _, fn := range f.funcs {
				funcnames = append(funcnames, fn.name)
				haveValues := index.QueryFunc(filename, fn.name)
				wantValues := HeatLevel{Local: int(fn.maxLocalLevel), Global: int(fn.maxGlobalLevel)}
				if haveValues != wantValues {
					t.Fatalf("QueryFunc(%s, %s): invalid heat values", filename, fn.name)
				}
			}
			if !sort.IsSorted(sort.StringSlice(funcnames)) {
				t.Fatalf("%s funcs are not sorted correctly", filename)
			}
			data := index.dataPoints[f.dataFrom:f.dataTo]
			linesSlice := make([]int, 0, len(data))
			for _, pt := range data {
				linesSlice = append(linesSlice, int(pt.line))
				haveValues := index.QueryLine(filename, int(pt.line))
				wantValues := HeatLevel{Local: pt.flags.GetLocalLevel(), Global: pt.flags.GetGlobalLevel()}
				if haveValues != wantValues {
					t.Fatalf("QueryLine(%s, %d): invalid heat values\nhave: %#v\nwant: %#v",
						filename, pt.line, haveValues, wantValues)
				}
				var haveValues2 HeatLevel
				index.queryLineRange(filename, int(pt.line), int(pt.line), func(line int, l HeatLevel) bool {
					haveValues2 = l
					if line != int(pt.line) {
						t.Fatalf("incorrect line from the queryLineRange()")
					}
					return true
				})
				if haveValues2 != wantValues {
					t.Fatalf("QueryLineRange(%s, %d, %d): invalid heat values\nhave: %#v\nwant: %#v",
						filename, pt.line, pt.line, haveValues2, wantValues)
				}
				var haveValues3 HeatLevel
				index.queryLineRange(filename, int(pt.line), int(pt.line), func(line int, l HeatLevel) bool {
					haveValues3 = l
					return true
				})
				if haveValues3 != wantValues {
					t.Fatalf("queryLineRange(%s, %d, %d): invalid heat values\nhave: %#v\nwant: %#v",
						filename, pt.line, pt.line, haveValues3, wantValues)
				}
				haveTotal := 0
				index.queryLineRange(filename, 1, int(f.maxLine), func(line int, l HeatLevel) bool {
					haveTotal++
					return true
				})
				if haveTotal != f.NumPoints() {
					t.Fatalf("queryLineRange(%s, 1, %d): results number mismatch\nhave: %v\nwant: %v",
						filename, int(f.maxLine), haveTotal, f.NumPoints())
				}
			}
			if f.minLine > f.maxLine {
				t.Fatalf("%s minLine > maxLine", filename)
			}
			if !sort.IsSorted(sort.IntSlice(linesSlice)) {
				t.Fatalf("%s data points are not sorted correctly", filename)
			}
		}
	}

	run := func(t *testing.T, name string, test testCase) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			p := test.buildProfile()
			index := NewIndex(test.config)
			if err := index.AddProfile(p); err != nil {
				t.Fatal(err)
			}
			validateIndex(t, index)
			have := dumpIndex(index)
			want := test.want
			if diff := cmp.Diff(have, want); diff != "" {
				t.Errorf("results mismatch:\n(+want -have)\n%s", diff)
			}
			for _, q := range test.queries {
				have := index.QueryLine(q.filename, q.line)
				want := q.want
				if diff := cmp.Diff(have, want); diff != "" {
					t.Errorf("QueryLine(%q, %d) results:\n(+want -have)\n%s", q.filename, q.line, diff)
				}
			}
			for _, q := range test.rangeQueries {
				have := []HeatLevel{}
				index.queryLineRange(q.filename, q.fromLine, q.toLine, func(line int, l HeatLevel) bool {
					have = append(have, l)
					return true
				})
				want := q.want
				if diff := cmp.Diff(have, want); diff != "" {
					t.Errorf("QueryLineRange(%q, %d, %d) results:\n(+want -have)\n%s", q.filename, q.fromLine, q.toLine, diff)
				}
			}
		})
	}

	for i := range tests {
		test := tests[i]

		// Running the test several times with randomized profile
		// samples order.
		for j := 0; j < 2; j++ {
			run(t, fmt.Sprintf("test%d_%d", i, j), test)
		}
	}
}

type testProfileBuilder struct {
	samples map[string][]testProfileSample
	sorted  bool
}

type testProfileSample struct {
	value int
	lines []int
}

func newTestProfileBuilder() *testProfileBuilder {
	return &testProfileBuilder{
		samples: make(map[string][]testProfileSample, 100),
	}
}

func (b *testProfileBuilder) Sorted() *testProfileBuilder {
	b.sorted = true
	return b
}

func (b *testProfileBuilder) AddSamples(sym string, pairs ...interface{}) *testProfileBuilder {
	if len(pairs)%2 != 0 {
		panic("odd number of arguments")
	}
	list := make([]testProfileSample, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		value := pairs[i+0].(int)
		lines := pairs[i+1].([]int)
		list = append(list, testProfileSample{value: value, lines: lines})
	}
	b.samples[sym] = append(b.samples[sym], list...)
	return b
}

func (b *testProfileBuilder) Build() *profile.Profile {
	p := &profile.Profile{
		SampleType: []*profile.ValueType{
			{Type: "samples", Unit: "count"},
			{Type: "cpu", Unit: "nanoseconds"},
		},
	}

	funcs := map[string]*profile.Function{}
	newSample := func() *profile.Sample {
		return &profile.Sample{
			Location: []*profile.Location{
				{},
			},
		}
	}
	getFunction := func(filename, funcName string) *profile.Function {
		k := filename + "/" + funcName
		f, ok := funcs[k]
		if !ok {
			f = &profile.Function{
				Name:     funcName,
				Filename: filename,
			}
			funcs[k] = f
		}
		return f
	}

	var outSamples []*profile.Sample
	for sym, symSampleSet := range b.samples {
		funcName := path.Base(sym)
		filename := path.Dir(sym)
		f := getFunction(filename, funcName)

		for _, s := range symSampleSet {
			pprofSample := newSample()
			pprofSample.Value = []int64{0, int64(s.value)}
			dstLoc := pprofSample.Location[0]
			outSamples = append(outSamples, pprofSample)
			for _, line := range s.lines {
				dstLoc.Line = append(dstLoc.Line, profile.Line{
					Line:     int64(line),
					Function: f,
				})
			}
		}
	}

	p.Sample = outSamples

	if b.sorted {
		for _, s := range p.Sample {
			for _, loc := range s.Location {
				sort.Slice(loc.Line, func(i, j int) bool {
					return loc.Line[i].Function.Name < loc.Line[j].Function.Name
				})
			}
		}
		sort.Slice(p.Sample, func(i, j int) bool {
			return p.Sample[i].Value[1] > p.Sample[j].Value[1]
		})
	} else {
		rand.Seed(time.Now().UnixNano())
		for _, s := range p.Sample {
			for _, loc := range s.Location {
				rand.Shuffle(len(loc.Line), func(i, j int) {
					loc.Line[i], loc.Line[j] = loc.Line[j], loc.Line[i]
				})
			}
		}
		rand.Shuffle(len(p.Sample), func(i, j int) {
			p.Sample[i], p.Sample[j] = p.Sample[j], p.Sample[i]
		})
	}

	return p
}
