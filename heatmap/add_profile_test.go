package heatmap

import (
	"fmt"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/pprof/profile"
)

func TestAddProfile(t *testing.T) {
	type sampleSet struct {
		funcname string
		value    int
		lines    []int
	}

	newFuncSampleSet := func(funcname string, samples ...sampleSet) []sampleSet {
		result := make([]sampleSet, len(samples))
		copy(result, samples)
		for i := range result {
			result[i].funcname = funcname
		}
		return result
	}
	newSampleSet := func(value int, lines []int) sampleSet {
		return sampleSet{value: value, lines: lines}
	}
	joinSamples := func(sets ...[]sampleSet) []sampleSet {
		var result []sampleSet
		for _, set := range sets {
			result = append(result, set...)
		}
		return result
	}

	createProfile := func(allSamples []sampleSet) *profile.Profile {
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
		for _, set := range allSamples {
			funcName := path.Base(set.funcname)
			filename := path.Dir(set.funcname)
			current := newSample()
			current.Value = []int64{0, int64(set.value)}
			loc := current.Location[0]
			outSamples = append(outSamples, current)
			f := getFunction(filename, funcName)
			for _, l := range set.lines {
				loc.Line = append(loc.Line, profile.Line{
					Line:     int64(l),
					Function: f,
				})
			}
		}
		p.Sample = outSamples
		return p
	}

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

	type testCase struct {
		samples   []sampleSet
		config    IndexConfig
		want      []string
		noReverse bool
	}

	tests := []testCase{
		{
			samples: joinSamples(
				newFuncSampleSet("buffer.go/example",
					newSampleSet(75, []int{10}),
					newSampleSet(25, []int{10})),
			),
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"func example (L=5 G=5)",
				"buffer.go:10: V=100 L=5 G=5",
			},
		},

		{
			samples: joinSamples(
				newFuncSampleSet("buffer.go/example",
					newSampleSet(75, []int{11, 12}),
					newSampleSet(25, []int{10})),
			),
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"func example (L=5 G=5)",
				"buffer.go:10: V= 25 L=0 G=0",
				"buffer.go:11: V= 75 L=0 G=0",
				"buffer.go:12: V= 75 L=5 G=5",
			},
		},

		{
			samples: joinSamples(
				newFuncSampleSet("buffer.go/example",
					newSampleSet(10, []int{5}),
					newSampleSet(11, []int{4}),
					newSampleSet(12, []int{3}),
					newSampleSet(13, []int{2}),
					newSampleSet(14, []int{1})),
			),
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
			samples: joinSamples(
				newFuncSampleSet("buffer.go/example",
					newSampleSet(10, []int{5}),
					newSampleSet(11, []int{4}),
					newSampleSet(12, []int{3}),
					newSampleSet(13, []int{2}),
					newSampleSet(14, []int{1})),
			),
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
			samples: joinSamples(
				newFuncSampleSet("a.go/f2",
					newSampleSet(100, []int{1, 2, 3}),
					newSampleSet(50, []int{2, 3}),
					newSampleSet(25, []int{3})),
				newFuncSampleSet("a.go/f1",
					newSampleSet(150, []int{6}),
					newSampleSet(160, []int{6}),
					newSampleSet(80, []int{10}),
					newSampleSet(40, []int{11})),
				newFuncSampleSet("b.go/f",
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6})),
			),
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
			samples: joinSamples(
				newFuncSampleSet("a.go/f1",
					newSampleSet(100, []int{1, 2, 3}),
					newSampleSet(50, []int{2, 3}),
					newSampleSet(25, []int{3}),
					newSampleSet(500, []int{4})),
				newFuncSampleSet("a.go/f2",
					newSampleSet(150, []int{6}),
					newSampleSet(160, []int{6}),
					newSampleSet(80, []int{10}),
					newSampleSet(40, []int{11})),
				newFuncSampleSet("b.go/f",
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6})),
			),
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
			samples: joinSamples(
				newFuncSampleSet("a.go/f1",
					newSampleSet(100, []int{1, 2, 3}),
					newSampleSet(50, []int{2, 3}),
					newSampleSet(25, []int{3}),
					newSampleSet(500, []int{4})),
				newFuncSampleSet("a.go/f2",
					newSampleSet(150, []int{6}),
					newSampleSet(200, []int{6}),
					newSampleSet(80, []int{10}),
					newSampleSet(40, []int{11})),
				newFuncSampleSet("b.go/f",
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{5, 6}),
					newSampleSet(40, []int{7}),
					newSampleSet(145, []int{7, 6, 5}),
					newSampleSet(40, []int{5, 6})),
			),
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
			samples: joinSamples(
				newFuncSampleSet("a.go/f1", newSampleSet(109, []int{1})),
				newFuncSampleSet("a.go/f2", newSampleSet(108, []int{2})),
				newFuncSampleSet("a.go/f3", newSampleSet(107, []int{3})),
				newFuncSampleSet("a.go/f4", newSampleSet(106, []int{4})),
				newFuncSampleSet("a.go/f5", newSampleSet(105, []int{5})),
				newFuncSampleSet("a.go/f6", newSampleSet(104, []int{6})),
				newFuncSampleSet("a.go/f7", newSampleSet(103, []int{7})),
				newFuncSampleSet("a.go/f8", newSampleSet(102, []int{8})),
				newFuncSampleSet("a.go/f9", newSampleSet(101, []int{9})),
			),
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
			noReverse: true,
			samples: joinSamples(
				newFuncSampleSet("a.go/f1", newSampleSet(109, []int{1})),
				newFuncSampleSet("a.go/f2", newSampleSet(108, []int{1})),
				newFuncSampleSet("a.go/f4", newSampleSet(106, []int{1})),
				newFuncSampleSet("a.go/f3", newSampleSet(107, []int{1})),
				newFuncSampleSet("a.go/f6", newSampleSet(104, []int{1})),
				newFuncSampleSet("a.go/f9", newSampleSet(101, []int{1})),
				newFuncSampleSet("a.go/f5", newSampleSet(105, []int{1})),
				newFuncSampleSet("a.go/f8", newSampleSet(102, []int{1})),
				newFuncSampleSet("a.go/f7", newSampleSet(103, []int{1})),
			),
			config: IndexConfig{Threshold: 1},
			want: []string{
				"func f1 (L=5 G=5)",
				"a.go:1: V=945 L=5 G=5",
			},
		},

		{
			samples: joinSamples(
				newFuncSampleSet("a.go/f1", newSampleSet(109, []int{5})),
				newFuncSampleSet("a.go/f2", newSampleSet(108, []int{6})),
				newFuncSampleSet("a.go/f3", newSampleSet(107, []int{7})),
				newFuncSampleSet("a.go/f4", newSampleSet(106, []int{1})),
				newFuncSampleSet("a.go/f5", newSampleSet(105, []int{2})),
				newFuncSampleSet("a.go/f6", newSampleSet(104, []int{3})),
				newFuncSampleSet("a.go/f7", newSampleSet(103, []int{4})),
				newFuncSampleSet("a.go/f8", newSampleSet(102, []int{8})),
				newFuncSampleSet("a.go/f9", newSampleSet(101, []int{9})),
			),
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
	}

	run := func(t *testing.T, name string, test testCase) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			p := createProfile(test.samples)
			index := NewIndex(test.config)
			if err := index.AddProfile(p); err != nil {
				t.Fatal(err)
			}
			have := dumpIndex(index)
			want := test.want
			if diff := cmp.Diff(have, want); diff != "" {
				t.Errorf("results mismatch:\n(+want -have)\n%s", diff)
			}
		})
	}

	for i := range tests {
		test := tests[i]

		run(t, fmt.Sprintf("test%d", i), test)

		if !test.noReverse {
			for i, j := 0, len(test.samples)-1; i < j; i, j = i+1, j-1 {
				test.samples[i], test.samples[j] = test.samples[j], test.samples[i]
			}
			run(t, fmt.Sprintf("test%drev", i), test)
		}
	}
}
