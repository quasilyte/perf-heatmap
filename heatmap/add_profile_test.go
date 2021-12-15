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

	funcNameSample := func(name string) sampleSet {
		return sampleSet{funcname: name}
	}
	newSampleSet := func(value int, lines []int) sampleSet {
		return sampleSet{value: value, lines: lines}
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
		funcName := ""
		filename := ""
		for _, set := range allSamples {
			if set.funcname != "" {
				funcName = path.Base(set.funcname)
				filename = path.Dir(set.funcname)
				continue
			}
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
			index.InspectFile(filename, func(s LineStats) {
				l := fmt.Sprintf("%s:%d: V=%3d L=%d G=%d", filename, s.LineNum, s.Value, s.HeatLevel, s.GlobalHeatLevel)
				lines = append(lines, l)
			})
		}
		return lines
	}

	tests := []struct {
		samples []sampleSet
		config  IndexConfig
		want    []string
	}{
		{
			samples: []sampleSet{
				funcNameSample("buffer.go/example"),
				newSampleSet(75, []int{10}),
				newSampleSet(25, []int{10}),
			},
			config: IndexConfig{Threshold: 0.25},
			want:   []string{"buffer.go:10: V=100 L=5 G=5"},
		},
		{
			samples: []sampleSet{
				funcNameSample("buffer.go/example"),
				newSampleSet(75, []int{11, 12}),
				newSampleSet(25, []int{10}),
			},
			config: IndexConfig{Threshold: 0.25},
			want: []string{
				"buffer.go:10: V= 25 L=0 G=0",
				"buffer.go:11: V= 75 L=0 G=0",
				"buffer.go:12: V= 75 L=5 G=5",
			},
		},
		{
			samples: []sampleSet{
				funcNameSample("buffer.go/example"),
				newSampleSet(10, []int{5}),
				newSampleSet(11, []int{4}),
				newSampleSet(12, []int{3}),
				newSampleSet(13, []int{2}),
				newSampleSet(14, []int{1}),
			},
			config: IndexConfig{Threshold: 1},
			want: []string{
				"buffer.go:1: V= 14 L=5 G=5",
				"buffer.go:2: V= 13 L=4 G=4",
				"buffer.go:3: V= 12 L=3 G=3",
				"buffer.go:4: V= 11 L=2 G=2",
				"buffer.go:5: V= 10 L=1 G=1",
			},
		},
		{
			samples: []sampleSet{
				funcNameSample("buffer.go/example"),
				newSampleSet(10, []int{5}),
				newSampleSet(11, []int{4}),
				newSampleSet(12, []int{3}),
				newSampleSet(13, []int{2}),
				newSampleSet(14, []int{1}),
			},
			config: IndexConfig{Threshold: 0.6},
			want: []string{
				"buffer.go:1: V= 14 L=5 G=5",
				"buffer.go:2: V= 13 L=4 G=4",
				"buffer.go:3: V= 12 L=3 G=3",
				"buffer.go:4: V= 11 L=0 G=0",
				"buffer.go:5: V= 10 L=0 G=0",
			},
		},
	}

	for _, test := range tests {
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
	}
}
