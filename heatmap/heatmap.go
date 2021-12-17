package heatmap

import (
	"sort"

	"github.com/google/pprof/profile"
)

// Index represents a parsed profile that can run heatmap queries efficiently.
//
// Index is not thread-safe.
// Adding profiles / querying index requires synchronization.
type Index struct {
	byFilename map[string]*fileIndex

	// A combined storage for every file index data points.
	// To get a file-specificic data points, do the slicing like
	// dataPoints[f.dataFrom:f.dataTo].
	//
	// This per-file window is sorted by dataPoint line field
	// in ascending order, so window[0].line <= window[1].line.
	dataPoints []dataPoint

	config IndexConfig
}

type IndexConfig struct {
	// TrimPrefix is a filename prefix to be trimmed from all locations.
	// If used, all filename arguments to the index need to trim this prefix too.
	TrimPrefix string

	// Threshold specifies where is the line between the "cold" and "hot" code.
	// Zero value implies 0.5, not 0.
	//
	// The threshould value can be interpreted in this way: what percentage
	// of top sample records we're marking as hot. For 0.5 it's top 50% results.
	// A value of 1.0 would includes all results.
	//
	// Threshould should be in the (0, 1.0] range.
	//
	// After the sample is included into the index, it'll be assigned the
	// "heat level". Values that are very close to the lower bound would get
	// a heat level of 1. The top-1 value always gets the level of 5.
	// Values in between get appropriate levels based on their distance.
	// Samples below the threshold may still end up populating the index,
	// but their heat level is guaranteed to be 0.
	//
	// There could be implementation details for edge cases.
	// For example, for files with a low number of samples we may
	// take all of them.
	Threshold float64
}

// FuncInfo contains some aggregated function info.
type FuncInfo struct {
	Name string

	MaxHeatLevel int

	MaxGlobalHeatLevel int
}

type HeatLevel struct {
	Local  int
	Global int
}

// NewIndex creates an empty heatmap index.
// Use AddProfile method to populate it.
func NewIndex(config IndexConfig) *Index {
	if config.Threshold == 0 {
		config.Threshold = 0.5
	}
	if config.Threshold <= 0 || config.Threshold > 1 {
		panic("IndexConfig.Threshold should be in (0, 1.0] range")
	}
	return &Index{config: config}
}

// AddProfile adds samples from the profile to the index.
// In the simplest use case, index only contains one profile.
//
// Adding samples with different labels/metrics is an error.
//
// This operation can take a long time.
func (index *Index) AddProfile(p *profile.Profile) error {
	return addProfile(index, p)
}

func (index *Index) CollectFilenames() []string {
	filenames := make([]string, 0, len(index.byFilename))
	for filename := range index.byFilename {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)
	return filenames
}

type LineStats struct {
	LineNum int

	// Value is the aggregate
	Value int64

	// HeatLevel is a file-local heat score according to the index settings.
	//
	// 0 means "cold": this line either didn't appear in the benchmark,
	// or it was below the specified threshold.
	//
	// Non-cold levels go from 1 to 5 (inclusive) with
	// 5 being the hottest level.
	HeatLevel int

	// GlobalHeatLevel is like HeatLevel, but it shows the score
	// based on global stats, not just file-local stats.
	// For example, some file may have lines with high HeatLevel,
	// but these lines may be irrelevant in the global picture.
	// GlobalHeatLevel is based on the aggregated top among all files.
	GlobalHeatLevel int

	// Func is a containing function info.
	Func *FuncInfo
}

// HasFile reports whether index contains the file.
func (index *Index) HasFile(filename string) bool {
	_, ok := index.byFilename[filename]
	return ok
}

// InspectFileLines visits all file data points using the provided callback.
// To check whether index has a file, use HasFile().
// To get all files contained inside the index, use CollectFilenames().
func (index *Index) InspectFileLines(filename string, visit func(LineStats)) {
	f := index.byFilename[filename]
	if f == nil {
		return
	}
	data := index.dataPoints[f.dataFrom:f.dataTo]
	var funcInfo FuncInfo
	for _, pt := range data {
		fn := &f.funcs[pt.funcIndex]
		funcInfo.Name = fn.name
		funcInfo.MaxHeatLevel = int(fn.maxLocalLevel)
		funcInfo.MaxGlobalHeatLevel = int(fn.maxGlobalLevel)
		visit(LineStats{
			LineNum:         int(pt.line),
			Value:           pt.value,
			HeatLevel:       pt.flags.GetLocalLevel(),
			GlobalHeatLevel: pt.flags.GetGlobalLevel(),
			Func:            &funcInfo,
		})
	}
}

func (index *Index) QueryFunc(filename, funcName string) HeatLevel {
	var result HeatLevel
	f, ok := index.byFilename[filename]
	if !ok {
		return result
	}
	i := sort.Search(len(f.funcs), func(i int) bool {
		return f.funcs[i].name >= funcName
	})
	if i < len(f.funcs) && f.funcs[i].name == funcName {
		fn := &f.funcs[i]
		result.Local = int(fn.maxLocalLevel)
		result.Global = int(fn.maxGlobalLevel)
	}
	return result
}

// QueryLineRange scans the file data points that are located in [lineFrom, lineTo] range.
// fn is called for every matching data point.
// Returning false from the callback causes the iteration to stop early.
func (index *Index) QueryLineRange(filename string, lineFrom, lineTo int, fn func(line int, level HeatLevel) bool) {
	if lineFrom == lineTo {
		fn(lineFrom, index.QueryLine(filename, lineFrom))
		return
	}
	index.queryLineRange(filename, lineFrom, lineTo, fn)
}

func (index *Index) QueryLine(filename string, line int) HeatLevel {
	var result HeatLevel
	f, ok := index.byFilename[filename]
	if !ok {
		return result
	}

	// A quick range check to avoid the search.
	if line < int(f.minLine) || line > int(f.maxLine) {
		return result
	}

	data := index.dataPoints[f.dataFrom:f.dataTo]
	if len(data) <= 4 {
		// Short data slice, use a linear search.
		for i := range data {
			pt := &data[i]
			if pt.line == uint32(line) {
				result = pt.HeatLevel()
				break
			}
		}
	} else {
		// Use a binary search for bigger data slices.
		i := sort.Search(len(data), func(i int) bool {
			return data[i].line >= uint32(line)
		})
		if i < len(data) && data[i].line == uint32(line) {
			result = data[i].HeatLevel()
		}
	}

	return result
}

func (index *Index) queryLineRange(filename string, lineFrom, lineTo int, fn func(line int, level HeatLevel) bool) {
	if lineFrom > lineTo {
		panic("lineFrom > lineTo")
	}

	f, ok := index.byFilename[filename]
	if !ok {
		return
	}

	// A quick range check to avoid the search.
	if int(f.maxLine) < lineFrom || int(f.minLine) > lineTo {
		return
	}

	// Narrow the search window by the data points range.
	if int(f.minLine) > lineFrom {
		lineFrom = int(f.minLine)
	}
	if int(f.maxLine) < lineTo {
		lineTo = int(f.maxLine)
	}

	data := index.dataPoints[f.dataFrom:f.dataTo]

	// It's possible to optimize the case where f.minLine=lineFrom && f.maxLine=lineTo,
	// where we would just walk the entire data slice, but
	// that use case doesn't look compelling enough to add an extra branch to the code.
	i := sort.Search(len(data), func(i int) bool {
		return data[i].line >= uint32(lineFrom)
	})
	if i < len(data) && data[i].line >= uint32(lineFrom) && data[i].line <= uint32(lineTo) {
		pt := &data[i]
		// i is a first matching entry, the leftmost one.
		if !fn(int(pt.line), pt.HeatLevel()) {
			return
		}
		// All data points until lineTo are matched too.
		for j := i + 1; j < len(data) && data[j].line <= uint32(lineTo); j++ {
			pt := &data[j]
			if !fn(int(pt.line), pt.HeatLevel()) {
				return
			}
		}
	}
}
