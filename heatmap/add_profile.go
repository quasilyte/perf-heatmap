package heatmap

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/pprof/profile"
)

func addProfile(index *Index, p *profile.Profile) error {
	w := &profileWalker{
		index: index,
		p:     p,
	}
	return w.Walk()
}

func trimLambdaSuffix(s string) string {
	end := len(s) - 1
	for {
		i := end
		for s[i] >= '0' && s[i] <= '9' {
			i--
		}
		found := false
		if strings.HasSuffix(s[:i+1], ".func") {
			i -= len(".func")
			found = true
		} else if s[i] == '.' {
			i--
			found = true
		}
		if !found {
			break
		}
		end = i
	}
	return s[:end+1]
}

func parseFuncName(s string) (pkgName, typeName, funcName string) {
	lastSlash := strings.LastIndexByte(s, '/')
	if lastSlash != -1 {
		s = s[lastSlash+len("/"):]
	}

	i := strings.IndexByte(s, '.')
	if i == -1 {
		return "", "", s
	}
	resultPkgName := s[:i]
	rest := s[i+1:]
	if strings.HasPrefix(rest, "(") {
		offset := 1
		if strings.HasPrefix(rest, "(*") {
			offset++
		}
		rparen := strings.IndexByte(rest, ')')
		if rparen == -1 {
			return "", "", ""
		}
		resultTypeName := rest[offset:rparen]
		resultFuncName := rest[rparen+len(")."):]
		return resultPkgName, resultTypeName, trimLambdaSuffix(resultFuncName)
	}
	return resultPkgName, "", trimLambdaSuffix(rest)
}

type profileWalker struct {
	index *Index
	p     *profile.Profile
}

func (w *profileWalker) Walk() error {
	// TODO: implement profiles merging?
	if w.index.funcIDByKey != nil {
		return errors.New("unimplemented yet: adding several profiles")
	}

	// TODO: support other kinds of profiles, like heap allocs?
	if len(w.p.SampleType) != 2 {
		return errors.New("unexpected profile type")
	}
	switch w.p.SampleType[1].Type + "/" + w.p.SampleType[1].Unit {
	case "cpu/nanoseconds":
		// OK.
	default:
		return fmt.Errorf("can't handle %s/%s samples yet", w.p.SampleType[1].Type, w.p.SampleType[1].Unit)
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

	type funcIndexTemplate struct {
		funcIndex
		origFilename string
		key          Key
		dataByLine   map[int64]dataPoint
	}

	// Step 1: aggregate the samples, build intermediate mappings.
	numDataPoints := uint64(0)
	filenameSet := map[string]uint32{}
	m := map[Key]*funcIndexTemplate{}
	for _, s := range w.p.Sample {
		sampleValue := s.Value[1]
		for _, loc := range s.Location {
			for _, l := range loc.Line {
				pkgName, typeName, funcName := parseFuncName(l.Function.Name)
				if pkgName == "" {
					continue
				}
				lineNum := l.Line
				if lineNum > math.MaxUint32 {
					continue
				}
				origFilename := l.Function.Filename
				filenameSet[origFilename] = 0 // Will be set to an actual index later
				key := Key{
					TypeName: typeName,
					FuncName: funcName,
					PkgName:  pkgName,
					Filename: filepath.Base(origFilename),
				}
				fn := m[key]
				if fn == nil {
					fn = &funcIndexTemplate{
						funcIndex: funcIndex{
							minLine: math.MaxUint32,
						},
						key:          key,
						origFilename: origFilename,
						dataByLine:   map[int64]dataPoint{},
					}
					m[key] = fn
				}
				pt, ok := fn.dataByLine[lineNum]
				if !ok {
					numDataPoints++
					pt.line = uint32(lineNum)
				}
				pt.value += sampleValue
				fn.dataByLine[lineNum] = pt
			}
		}
	}

	if numDataPoints == 0 {
		return errors.New("found no suitable samples")
	}
	if numDataPoints > math.MaxUint32 {
		return fmt.Errorf("too many samples (%d)", numDataPoints)
	}

	// Step 2: sort all filenames.
	sortedFilenames := make([]string, 0, len(filenameSet))
	for filename := range filenameSet {
		sortedFilenames = append(sortedFilenames, filename)
	}
	sort.Strings(sortedFilenames)
	for i, filename := range sortedFilenames {
		filenameSet[filename] = uint32(i)
	}

	// Step 3: sort all functions.
	funcs := make([]*funcIndexTemplate, 0, len(m))
	for _, fn := range m {
		fn.fileID = filenameSet[fn.origFilename]
		funcs = append(funcs, fn)
	}
	sort.Slice(funcs, func(i, j int) bool {
		f1 := funcs[i]
		f2 := funcs[j]
		if f1.origFilename != f2.origFilename {
			return f1.origFilename < f2.origFilename
		}
		if f1.key.TypeName != f2.key.TypeName {
			return f1.key.TypeName < f2.key.TypeName
		}
		return f1.key.FuncName < f2.key.FuncName
	})

	// Step 4: put all aggregated points into one slice, bind data ranges to files.
	allPoints := make([]dataPoint, 0, numDataPoints)
	for _, fn := range funcs {
		fn.dataFrom = uint32(len(allPoints))
		for _, pt := range fn.dataByLine {
			allPoints = append(allPoints, pt)
			if pt.line > fn.maxLine {
				fn.maxLine = pt.line
			}
			if pt.line < fn.minLine {
				fn.minLine = pt.line
			}
		}
		fn.dataTo = uint32(len(allPoints))

		// Compute local heat levels.
		funcData := allPoints[fn.dataFrom:fn.dataTo]
		sort.Slice(funcData, func(i, j int) bool {
			return pointGreater(funcData[i], funcData[j])
		})
		topn := int(float64(len(funcData)) * w.index.config.Threshold)
		if topn == 0 {
			topn = 1
		}
		points := funcData[:topn]
		currentLevel := 5
		currentChunk := 0
		forChunks(len(points), maxHeatLevel, func(chunkNum, i int) {
			pt := &points[i]
			if currentChunk != chunkNum {
				currentLevel--
				currentChunk = chunkNum
			}
			pt.flags.SetLocalLevel(currentLevel)
		})
		// A final sort: by line.
		sort.Slice(funcData, func(i, j int) bool {
			return funcData[i].line < funcData[j].line
		})
	}

	// Step 5: compute the global heat levels.
	valueOrder := make([]uint32, len(allPoints))
	for i := range allPoints {
		valueOrder[i] = uint32(i)
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
		currentLevel := 5
		currentChunk := 0
		forChunks(topn, maxHeatLevel, func(chunkNum, i int) {
			pt := &allPoints[valueOrder[i]]
			if currentChunk != chunkNum {
				currentLevel--
				currentChunk = chunkNum
			}
			pt.flags.SetGlobalLevel(currentLevel)
		})
	}

	w.index.filenames = sortedFilenames
	w.index.funcs = make([]funcIndex, len(funcs))
	w.index.funcIDByKey = map[Key]uint32{}
	w.index.dataPoints = allPoints
	for i, fn := range funcs {
		funcData := allPoints[fn.dataFrom:fn.dataTo]
		for i := range funcData {
			pt := &funcData[i]
			if pt.flags.GetLocalLevel() > int(fn.maxLocalLevel) {
				fn.maxLocalLevel = uint8(pt.flags.GetLocalLevel())
			}
			if pt.flags.GetGlobalLevel() > int(fn.maxGlobalLevel) {
				fn.maxGlobalLevel = uint8(pt.flags.GetGlobalLevel())
			}
		}
		w.index.funcs[i] = fn.funcIndex
		w.index.funcIDByKey[fn.key] = uint32(i)
	}

	return nil
}
