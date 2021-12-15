package heatmap

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/google/pprof/profile"
)

func addProfile(index *Index, p *profile.Profile) error {
	w := &profileWalker{
		index: index,
		p:     p,
	}
	return w.Walk()
}

type profileWalker struct {
	index *Index
	p     *profile.Profile
}

func (w *profileWalker) Walk() error {
	// TODO: implement profiles merging.
	if w.index.byFilename != nil {
		return errors.New("unimplemented yet: adding several profiles")
	}

	if len(w.p.SampleType) != 2 {
		return errors.New("unexpected profile type")
	}
	switch w.p.SampleType[1].Type + "/" + w.p.SampleType[1].Unit {
	case "cpu/nanoseconds":
		// OK.
	default:
		return fmt.Errorf("can't handle %s/%s samples yet", w.p.SampleType[1].Type, w.p.SampleType[1].Unit)
	}

	// Pass 1: aggregate the samples, build intermediate mappings.
	numDataPoints := uint64(0)
	m := map[string]*fileIndex{}
	fileValuesByName := map[string]map[int64]*dataPoint{}
	for _, s := range w.p.Sample {
		sampleValue := s.Value[1]
		for _, loc := range s.Location {
			for _, l := range loc.Line {
				filename := l.Function.Filename
				lineNum := l.Line
				f := m[filename]
				if f == nil {
					f = &fileIndex{
						minLine: math.MaxUint32,
					}
					m[filename] = f
				}
				fileValueByLine := fileValuesByName[filename]
				if fileValueByLine == nil {
					fileValueByLine = map[int64]*dataPoint{}
					fileValuesByName[filename] = fileValueByLine
				}
				pt := fileValueByLine[lineNum]
				if lineNum > math.MaxUint32 {
					continue
				}
				if pt == nil {
					numDataPoints++
					pt = &dataPoint{line: uint32(lineNum)}
					fileValueByLine[lineNum] = pt
				}
				pt.value += sampleValue
			}
		}
	}

	if numDataPoints == 0 {
		return errors.New("found no suitable samples")
	}
	if numDataPoints > math.MaxUint32 {
		return fmt.Errorf("too many samples (%d)", numDataPoints)
	}

	// Pass 2: put all aggregated points into one slice, bind data ranges to files.
	allPoints := make([]dataPoint, 0, numDataPoints)
	for filename, fileValueByLine := range fileValuesByName {
		f := m[filename]
		f.dataFrom = len(allPoints)
		for _, pt := range fileValueByLine {
			allPoints = append(allPoints, *pt)
			if pt.line > f.maxLine {
				f.maxLine = pt.line
			} else if pt.line < f.minLine {
				f.minLine = pt.line
			}
		}
		f.dataTo = len(allPoints)
	}

	// Pass 3: compute the global heat levels.
	valueOrder := make([]uint32, len(allPoints))
	for i := range allPoints {
		valueOrder[i] = uint32(i)
	}
	pointGreater := func(x, y dataPoint) bool {
		if x.value > y.value {
			return true
		}
		if x.value < y.value {
			return false
		}
		return x.line > y.line
	}
	sort.Slice(valueOrder, func(i, j int) bool {
		x := allPoints[valueOrder[i]]
		y := allPoints[valueOrder[j]]
		return pointGreater(x, y)
	})
	{
		topn := int(float64(numDataPoints) * w.index.config.Threshold)
		if topn == 0 {
			topn = 1
		}
		points := make([]dataPoint, topn)
		for i := range points {
			points[i] = allPoints[valueOrder[i]]
		}
		currentLevel := 5
		currentChunk := 0
		walkDataChunks(points, maxHeatLevel, func(chunkNum, i int) {
			pt := &allPoints[valueOrder[i]]
			if currentChunk != chunkNum {
				currentLevel--
				currentChunk = chunkNum
			}
			pt.flags.SetGlobalLevel(currentLevel)
		})
	}

	// Pass 4: apply a final sort for per-file windows. Also compute the local heat levels.
	for _, f := range m {
		data := allPoints[f.dataFrom:f.dataTo]

		sort.Slice(data, func(i, j int) bool {
			return pointGreater(data[i], data[j])
		})
		topn := int(float64(len(data)) * w.index.config.Threshold)
		if topn == 0 {
			topn = 1
		}
		points := data[:topn]
		currentLevel := 5
		currentChunk := 0
		walkDataChunks(points, maxHeatLevel, func(chunkNum, i int) {
			pt := &points[i]
			if currentChunk != chunkNum {
				currentLevel--
				currentChunk = chunkNum
			}
			pt.flags.SetLocalLevel(currentLevel)
		})
		// A final sort: by line.
		sort.Slice(data, func(i, j int) bool {
			return data[i].line < data[j].line
		})
	}

	w.index.byFilename = m
	w.index.dataPoints = allPoints

	return nil
}
