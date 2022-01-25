package heatmap

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/pprof/profile"
	"github.com/quasilyte/pprofutil"
)

func convertTestKey(s string) Key {
	var key Key
	parts := strings.Split(s, ":")
	key.Filename = parts[0]
	sym := pprofutil.ParseFuncName(parts[1])
	key.PkgName = sym.PkgName
	key.TypeName = sym.TypeName
	key.FuncName = sym.FuncName
	return key
}

func TestConvertTestKey(t *testing.T) {
	tests := []struct {
		s        string
		filename string
		pkgName  string
		typeName string
		funcName string
	}{
		{"file.go:pkg.f", "file.go", "pkg", "", "f"},
		{"file.go:pkg.(T).f", "file.go", "pkg", "T", "f"},
		{"file.go:pkg.(*T).f", "file.go", "pkg", "T", "f"},
		{"foo/file.go:pkg.(*T).f", "foo/file.go", "pkg", "T", "f"},
		{"/foo/file.go:pkg.(*T).f", "/foo/file.go", "pkg", "T", "f"},
	}

	for _, test := range tests {
		k := convertTestKey(test.s)
		if k.Filename != test.filename {
			t.Fatalf("convertTestKey(%q) filename => have %s, want %s", test.s, k.Filename, test.pkgName)
		}
		if k.PkgName != test.pkgName {
			t.Fatalf("convertTestKey(%q) pkgName => have %s, want %s", test.s, k.PkgName, test.pkgName)
		}
		if k.TypeName != test.typeName {
			t.Fatalf("convertTestKey(%q) typeName => have %s, want %s", test.s, k.TypeName, test.typeName)
		}
		if k.FuncName != test.funcName {
			t.Fatalf("convertTestKey(%q) funcName => have %s, want %s", test.s, k.FuncName, test.funcName)
		}
	}
}

func TestAddProfile(t *testing.T) {
	dumpIndex := func(index *Index) []string {
		var lines []string
		sortedKeys := make([]Key, 0, len(index.funcIDByKey))
		for key := range index.funcIDByKey {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Slice(sortedKeys, func(i, j int) bool {
			x := sortedKeys[i]
			y := sortedKeys[j]
			if x.PkgName != y.PkgName {
				return x.PkgName < y.PkgName
			}
			if x.Filename != y.Filename {
				return x.Filename < y.Filename
			}
			if x.TypeName != y.TypeName {
				return x.TypeName < y.TypeName
			}
			return x.FuncName < y.FuncName
		})
		for _, key := range sortedKeys {
			funcID := index.funcIDByKey[key]
			fn := &index.funcs[funcID]
			lines = append(lines, fmt.Sprintf("func %s (L=%d G=%d)",
				formatFuncName(key.PkgName, key.TypeName, key.FuncName), fn.maxLocalLevel, fn.maxGlobalLevel))
			data := index.dataPoints[fn.dataFrom:fn.dataTo]
			filename := index.filenames[fn.fileID]
			for i := range data {
				pt := &data[i]
				l := fmt.Sprintf("%s:%d: V=%3d L=%d G=%d",
					filename, pt.line, pt.value, pt.flags.GetLocalLevel(), pt.flags.GetGlobalLevel())
				lines = append(lines, l)
			}
		}
		return lines
	}

	type testQuery struct {
		key  string
		line int
		want LineStats
	}

	type testRangeQuery struct {
		key      string
		fromLine int
		toLine   int
		want     []LineStats
	}

	type testCase struct {
		buildProfile func() *profile.Profile
		config       IndexConfig
		want         []string
		queries      []testQuery
		rangeQueries []testRangeQuery
	}

	newStats := func(local, global int) LineStats {
		return LineStats{HeatLevel: local, GlobalHeatLevel: global}
	}

	tests := []testCase{
		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go:example.f", 25, []int{10}).
				AddSamples("buffer.go:example.f", 75, []int{10}).
				Build,
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"func example.f (L=5 G=5)",
				"buffer.go:10: V=100 L=5 G=5",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go:example.f",
					25, []int{10},
					75, []int{10}).
				Build,
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"func example.f (L=5 G=5)",
				"buffer.go:10: V=100 L=5 G=5",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("/home/gopher/buffer.go:example.fn",
					75, []int{10}).
				Build,
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"func example.fn (L=5 G=5)",
				"/home/gopher/buffer.go:10: V= 75 L=5 G=5",
			},
			queries: []testQuery{
				{"buffer.go:example.fn", 10, newStats(5, 5)},

				// Query using the invalid func name.
				{"buffer.go:example2.fn", 10, newStats(0, 0)},
				// Query at the invalid line.
				{"buffer.go:example.fn", 9, newStats(0, 0)},
				// Query using the invalid filename.
				{"buffer2.go:example.fn", 10, newStats(0, 0)},
			},
			rangeQueries: []testRangeQuery{
				{"buffer.go:example.fn", 9, 11, []LineStats{newStats(5, 5)}},
				{"buffer.go:example.fn", 6, 10, []LineStats{newStats(5, 5)}},
				{"buffer.go:example.fn", 6, 9, []LineStats{}},
				{"buffer.go:example.fn", 15, 20, []LineStats{}},
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go:pkg.example",
					75, []int{11, 12},
					25, []int{10}).
				Build,
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"func pkg.example (L=5 G=5)",
				"buffer.go:10: V= 25 L=0 G=0",
				"buffer.go:11: V= 75 L=0 G=0",
				"buffer.go:12: V= 75 L=5 G=5",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go:example.fff",
					10, []int{5},
					11, []int{4},
					12, []int{3},
					13, []int{2},
					14, []int{1}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func example.fff (L=5 G=5)",
				"buffer.go:1: V= 14 L=5 G=5",
				"buffer.go:2: V= 13 L=4 G=4",
				"buffer.go:3: V= 12 L=3 G=3",
				"buffer.go:4: V= 11 L=2 G=2",
				"buffer.go:5: V= 10 L=1 G=1",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go:example.example",
					10, []int{5},
					11, []int{4},
					12, []int{3},
					13, []int{2},
					14, []int{1}).
				Build,
			config: IndexConfig{Threshold: 0.6},
			want: []string{
				"func example.example (L=5 G=5)",
				"buffer.go:1: V= 14 L=5 G=5",
				"buffer.go:2: V= 13 L=4 G=4",
				"buffer.go:3: V= 12 L=3 G=3",
				"buffer.go:4: V= 11 L=0 G=0",
				"buffer.go:5: V= 10 L=0 G=0",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go:example.example",
					10, []int{5},
					11, []int{4},
					12, []int{3},
					13, []int{2},
					14, []int{1}).
				Build,
			config: IndexConfig{Threshold: 0.1},
			want: []string{
				"func example.example (L=5 G=5)",
				"buffer.go:1: V= 14 L=5 G=5",
				"buffer.go:2: V= 13 L=0 G=0",
				"buffer.go:3: V= 12 L=0 G=0",
				"buffer.go:4: V= 11 L=0 G=0",
				"buffer.go:5: V= 10 L=0 G=0",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("buffer.go:example.example",
					10, []int{5},
					11, []int{4},
					12, []int{3},
					13, []int{2},
					14, []int{1}).
				Build,
			config: IndexConfig{Threshold: 0.01},
			want: []string{
				"func example.example (L=5 G=5)",
				"buffer.go:1: V= 14 L=5 G=5",
				"buffer.go:2: V= 13 L=0 G=0",
				"buffer.go:3: V= 12 L=0 G=0",
				"buffer.go:4: V= 11 L=0 G=0",
				"buffer.go:5: V= 10 L=0 G=0",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go:pkg1.(*T).f2",
					100, []int{1, 2, 3},
					50, []int{2, 3},
					25, []int{3}).
				AddSamples("a.go:pkg1.(*T).f1",
					150, []int{6},
					160, []int{6},
					80, []int{10},
					40, []int{11}).
				AddSamples("b.go:pkg2.f",
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func pkg1.(T).f1 (L=5 G=5)",
				"a.go:6: V=310 L=5 G=5",
				"a.go:10: V= 80 L=4 G=1",
				"a.go:11: V= 40 L=3 G=1",

				"func pkg1.(T).f2 (L=5 G=3)",
				"a.go:1: V=100 L=3 G=2",
				"a.go:2: V=150 L=4 G=2",
				"a.go:3: V=175 L=5 G=3",

				"func pkg2.f (L=5 G=4)",
				"b.go:5: V=200 L=4 G=4",
				"b.go:6: V=200 L=5 G=4",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go:pkg.f1",
					100, []int{1, 2, 3},
					50, []int{2, 3},
					25, []int{3},
					500, []int{4}).
				AddSamples("a.go:pkg.f2",
					150, []int{6},
					160, []int{6},
					80, []int{10},
					40, []int{11}).
				AddSamples("b.go:pkg.f",
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6},
					40, []int{5, 6}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func pkg.f1 (L=5 G=5)",
				"a.go:1: V=100 L=2 G=2",
				"a.go:2: V=150 L=3 G=2",
				"a.go:3: V=175 L=4 G=3",
				"a.go:4: V=500 L=5 G=5",

				"func pkg.f2 (L=5 G=4)",
				"a.go:6: V=310 L=5 G=4",
				"a.go:10: V= 80 L=4 G=1",
				"a.go:11: V= 40 L=3 G=1",

				"func pkg.f (L=5 G=4)",
				"b.go:5: V=200 L=4 G=3",
				"b.go:6: V=200 L=5 G=4",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go:example.f1",
					100, []int{1, 2, 3},
					50, []int{2, 3},
					25, []int{3},
					500, []int{4}).
				AddSamples("a.go:example.f2",
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
				AddSamples("b.go:example.f",
					40, []int{5, 6},
					40, []int{5, 7},
					40, []int{5, 7},
					40, []int{5, 6},
					40, []int{5, 6}).
				AddSamples("c.go:example.f",
					1, []int{1},
					2, []int{1},
					3, []int{1}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func example.f1 (L=5 G=5)",
				"a.go:1: V=100 L=2 G=2",
				"a.go:2: V=150 L=3 G=3",
				"a.go:3: V=175 L=4 G=4",
				"a.go:4: V=500 L=5 G=5",

				"func example.f2 (L=5 G=5)",
				"a.go:6: V=310 L=5 G=5",
				"a.go:10: V= 80 L=2 G=2",
				"a.go:11: V= 40 L=1 G=1",
				"a.go:14: V=150 L=3 G=3",
				"a.go:15: V=160 L=4 G=4",
				"a.go:16: V=120 L=3 G=3",
				"a.go:17: V=150 L=4 G=4",
				"a.go:19: V=160 L=5 G=4",
				"a.go:24: V= 80 L=2 G=2",
				"a.go:28: V= 40 L=1 G=1",

				"func example.f (L=5 G=5)",
				"b.go:5: V=200 L=5 G=5",
				"b.go:6: V=120 L=4 G=2",
				"b.go:7: V= 80 L=3 G=1",

				"func example.f (L=5 G=1)",
				"c.go:1: V=  6 L=5 G=1",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go:test.f1.func1",
					100, []int{1, 2, 3},
					50, []int{2, 3},
					25, []int{3},
					500, []int{4}).
				AddSamples("a.go:test.f1.func2",
					150, []int{6},
					200, []int{6},
					80, []int{10},
					40, []int{11}).
				AddSamples("b.go:test.f",
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
				"func test.f1 (L=5 G=5)",
				"a.go:1: V=100 L=0 G=0",
				"a.go:2: V=150 L=0 G=0",
				"a.go:3: V=175 L=3 G=0",
				"a.go:4: V=500 L=5 G=5",
				"a.go:6: V=350 L=4 G=4",
				"a.go:10: V= 80 L=0 G=0",
				"a.go:11: V= 40 L=0 G=0",

				"func test.f (L=5 G=3)",
				"b.go:5: V=345 L=0 G=2",
				"b.go:6: V=345 L=5 G=3",
				"b.go:7: V=185 L=0 G=1",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go:test.f1", 109, []int{1}).
				AddSamples("a.go:test.f2", 108, []int{2}).
				AddSamples("a.go:test.f3", 107, []int{3}).
				AddSamples("a.go:test.f4", 106, []int{4}).
				AddSamples("a.go:test.f5", 105, []int{5}).
				AddSamples("a.go:test.f6", 104, []int{6}).
				AddSamples("a.go:test.f7", 103, []int{7}).
				AddSamples("a.go:test.f8", 102, []int{8}).
				AddSamples("a.go:test.f9", 101, []int{9}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func test.f1 (L=5 G=5)",
				"a.go:1: V=109 L=5 G=5",
				"func test.f2 (L=5 G=4)",
				"a.go:2: V=108 L=5 G=4",
				"func test.f3 (L=5 G=4)",
				"a.go:3: V=107 L=5 G=4",
				"func test.f4 (L=5 G=3)",
				"a.go:4: V=106 L=5 G=3",
				"func test.f5 (L=5 G=3)",
				"a.go:5: V=105 L=5 G=3",
				"func test.f6 (L=5 G=2)",
				"a.go:6: V=104 L=5 G=2",
				"func test.f7 (L=5 G=2)",
				"a.go:7: V=103 L=5 G=2",
				"func test.f8 (L=5 G=1)",
				"a.go:8: V=102 L=5 G=1",
				"func test.f9 (L=5 G=1)",
				"a.go:9: V=101 L=5 G=1",
			},
		},

		// All samples would point to the same func+line, resulting in a single data point.
		{
			buildProfile: newTestProfileBuilder().
				Sorted().
				AddSamples("/foo/go/src/a.go:test.f", 109, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f", 108, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f", 106, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f", 107, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f", 104, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f", 101, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f", 105, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f", 102, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f", 103, []int{1}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func test.f (L=5 G=5)",
				"/foo/go/src/a.go:1: V=945 L=5 G=5",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				Sorted().
				AddSamples("/foo/go/src/a.go:test.f1", 109, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f2", 108, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f3", 106, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f4", 107, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f5", 104, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f6", 101, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f7", 105, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f8", 102, []int{1}).
				AddSamples("/foo/go/src/a.go:test.f9", 103, []int{1}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func test.f1 (L=5 G=5)",
				"/foo/go/src/a.go:1: V=109 L=5 G=5",
				"func test.f2 (L=5 G=4)",
				"/foo/go/src/a.go:1: V=108 L=5 G=4",
				"func test.f3 (L=5 G=3)",
				"/foo/go/src/a.go:1: V=106 L=5 G=3",
				"func test.f4 (L=5 G=4)",
				"/foo/go/src/a.go:1: V=107 L=5 G=4",
				"func test.f5 (L=5 G=2)",
				"/foo/go/src/a.go:1: V=104 L=5 G=2",
				"func test.f6 (L=5 G=1)",
				"/foo/go/src/a.go:1: V=101 L=5 G=1",
				"func test.f7 (L=5 G=3)",
				"/foo/go/src/a.go:1: V=105 L=5 G=3",
				"func test.f8 (L=5 G=1)",
				"/foo/go/src/a.go:1: V=102 L=5 G=1",
				"func test.f9 (L=5 G=2)",
				"/foo/go/src/a.go:1: V=103 L=5 G=2",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("a.go:x.(Example).f1", 109, []int{5}).
				AddSamples("a.go:x.(Example).f2", 108, []int{6}).
				AddSamples("a.go:x.(Example).f3", 107, []int{7}).
				AddSamples("a.go:x.(Example).f4", 106, []int{1}).
				AddSamples("a.go:x.(Example).f5", 105, []int{2}).
				AddSamples("a.go:x.(Example).f6", 104, []int{3}).
				AddSamples("a.go:x.(Example).f7", 103, []int{4}).
				AddSamples("a.go:x.(Example).f8", 102, []int{8}).
				AddSamples("a.go:x.(Example).f9", 101, []int{9}).
				Build,
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func x.(Example).f1 (L=5 G=5)",
				"a.go:5: V=109 L=5 G=5",
				"func x.(Example).f2 (L=5 G=4)",
				"a.go:6: V=108 L=5 G=4",
				"func x.(Example).f3 (L=5 G=4)",
				"a.go:7: V=107 L=5 G=4",
				"func x.(Example).f4 (L=5 G=3)",
				"a.go:1: V=106 L=5 G=3",
				"func x.(Example).f5 (L=5 G=3)",
				"a.go:2: V=105 L=5 G=3",
				"func x.(Example).f6 (L=5 G=2)",
				"a.go:3: V=104 L=5 G=2",
				"func x.(Example).f7 (L=5 G=2)",
				"a.go:4: V=103 L=5 G=2",
				"func x.(Example).f8 (L=5 G=1)",
				"a.go:8: V=102 L=5 G=1",
				"func x.(Example).f9 (L=5 G=1)",
				"a.go:9: V=101 L=5 G=1",
			},
		},

		{
			buildProfile: newTestProfileBuilder().
				AddSamples("file.go:example.testfunc",
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
				"func example.testfunc (L=5 G=5)",
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
					key:      "file.go:example.testfunc",
					fromLine: 110,
					toLine:   150,
					want:     []LineStats{},
				},
				{
					key:      "file.go:example.testfunc",
					fromLine: 195,
					toLine:   205,
					want: []LineStats{
						newStats(3, 3),
						newStats(1, 1),
						newStats(4, 4),
					},
				},
				{
					key:      "file.go:example.testfunc",
					fromLine: 200,
					toLine:   205,
					want: []LineStats{
						newStats(3, 3),
						newStats(1, 1),
						newStats(4, 4),
					},
				},
				{
					key:      "file.go:example.testfunc",
					fromLine: 202,
					toLine:   205,
					want: []LineStats{
						newStats(4, 4),
					},
				},
			},
		},
	}

	ignoreFields := cmpopts.IgnoreFields(LineStats{}, "Value", "LineNum")
	statsDiff := func(x, y interface{}) string {
		return cmp.Diff(x, y, ignoreFields)
	}

	validateIndex := func(t *testing.T, index *Index) {
		if !sort.IsSorted(sort.StringSlice(index.filenames)) {
			t.Fatal("filenames are not sorted correctly")
		}

		eqStats := func(x, y LineStats) bool {
			return x.HeatLevel == y.HeatLevel && x.GlobalHeatLevel == y.GlobalHeatLevel
		}

		for key, funcID := range index.funcIDByKey {
			fn := index.funcs[funcID]
			filename := index.filenames[fn.fileID]
			data := index.dataPoints[fn.dataFrom:fn.dataTo]
			linesSlice := make([]int, 0, len(data))
			for _, pt := range data {
				linesSlice = append(linesSlice, int(pt.line))
				haveValues := index.QueryLine(key, int(pt.line))
				wantValues := LineStats{HeatLevel: pt.flags.GetLocalLevel(), GlobalHeatLevel: pt.flags.GetGlobalLevel()}
				if !eqStats(haveValues, wantValues) {
					t.Fatalf("QueryLine(%s, %d): invalid heat values\nhave: %#v\nwant: %#v",
						filename, pt.line, haveValues, wantValues)
				}
				var haveValues2 LineStats
				index.queryLineRange(key, int(pt.line), int(pt.line), func(l LineStats) bool {
					haveValues2 = l
					if l.LineNum != int(pt.line) {
						t.Fatalf("incorrect line from the queryLineRange()")
					}
					return true
				})
				if !eqStats(haveValues2, wantValues) {
					t.Fatalf("QueryLineRange(%s, %d, %d): invalid heat values\nhave: %#v\nwant: %#v",
						filename, pt.line, pt.line, haveValues2, wantValues)
				}
				var haveValues3 LineStats
				index.queryLineRange(key, int(pt.line), int(pt.line), func(l LineStats) bool {
					haveValues3 = l
					return true
				})
				if !eqStats(haveValues3, wantValues) {
					t.Fatalf("queryLineRange(%s, %d, %d): invalid heat values\nhave: %#v\nwant: %#v",
						filename, pt.line, pt.line, haveValues3, wantValues)
				}
				haveTotal := 0
				index.queryLineRange(key, 1, int(fn.maxLine), func(l LineStats) bool {
					haveTotal++
					return true
				})
				if haveTotal != fn.NumPoints() {
					t.Fatalf("queryLineRange(%s, 1, %d): results number mismatch\nhave: %v\nwant: %v",
						filename, int(fn.maxLine), haveTotal, fn.NumPoints())
				}
			}
			if fn.minLine > fn.maxLine {
				t.Fatalf("%s minLine > maxLine", filename)
			}
			if !sort.IsSorted(sort.IntSlice(linesSlice)) {
				t.Fatalf("%s data points are not sorted correctly", filename)
			}
		}
	}

	run := func(t *testing.T, name string, test testCase) {
		t.Run(name, func(t *testing.T) {
			p := test.buildProfile()
			index := NewIndex(test.config)
			if err := index.AddProfile(p); err != nil {
				t.Fatal(err)
			}
			validateIndex(t, index)
			have := dumpIndex(index)
			want := test.want
			if diff := statsDiff(have, want); diff != "" {
				t.Errorf("results mismatch:\n(+want -have)\n%s", diff)
			}
			for _, q := range test.queries {
				have := index.QueryLine(convertTestKey(q.key), q.line)
				want := q.want
				if diff := statsDiff(have, want); diff != "" {
					t.Errorf("QueryLine(%q, %d) results:\n(+want -have)\n%s", q.key, q.line, diff)
				}
			}
			for _, q := range test.rangeQueries {
				have := []LineStats{}
				index.queryLineRange(convertTestKey(q.key), q.fromLine, q.toLine, func(l LineStats) bool {
					have = append(have, l)
					return true
				})
				want := q.want
				if diff := statsDiff(have, want); diff != "" {
					t.Errorf("QueryLineRange(%q, %d, %d) results:\n(+want -have)\n%s", q.key, q.fromLine, q.toLine, diff)
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

	funcs := map[Key]*profile.Function{}
	newSample := func() *profile.Sample {
		return &profile.Sample{
			Location: []*profile.Location{
				{},
			},
		}
	}
	getFunction := func(key Key) *profile.Function {
		f, ok := funcs[key]
		if !ok {
			f = &profile.Function{
				Name:     formatFuncName(key.PkgName, key.TypeName, key.FuncName),
				Filename: key.Filename,
			}
			funcs[key] = f
		}
		return f
	}

	var outSamples []*profile.Sample
	for sym, symSampleSet := range b.samples {
		key := convertTestKey(sym)
		f := getFunction(key)

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
