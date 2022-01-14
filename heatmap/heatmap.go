package heatmap

import (
	"sort"

	"github.com/google/pprof/profile"
)

type Key struct {
	// TypeName is a receiver type name for methods.
	// For functions it should be empty.
	TypeName string

	// FuncName is a Go function name.
	// For methods, TypeName+FuncName compose a full method name.
	FuncName string

	// Filename is a base part of the full file path.
	// For the `/home/go/src/bytes/buffer.go` it would be just `buffer.go`.
	Filename string

	// PkgName is a symbol defining package name.
	PkgName string
}

// Index represents a parsed profile that can run heatmap queries efficiently.
type Index struct {
	funcIDByKey map[Key]uint32

	// A combined storage for all data points.
	// To get func-specificic data points, do the slicing like
	// dataPoints[fn.dataFrom:fn.dataTo].
	//
	// This per-func window is sorted by dataPoint line field
	// in ascending order, so window[0].line <= window[1].line.
	dataPoints []dataPoint

	funcs []funcIndex

	// filenames is a list of full file names.
	filenames []string

	config IndexConfig
}

type IndexConfig struct {
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
	ID string

	PkgName string

	Filename string

	MaxHeatLevel int

	MaxGlobalHeatLevel int
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
	return index.filenames
}

type LineStats struct {
	LineNum int

	// Value is the aggregated profile samples value for this line.
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
	// Note: it will be nil for Query functions.
	Func *FuncInfo
}

// Inspect visits all data points using the provided callback.
//
// The data points traversal order is not deterministic, but
// it's guaranteed to walk func-associated data points in
// source line sorted order.
func (index *Index) Inspect(callback func(LineStats)) {
	var funcInfo FuncInfo
	for key, funcID := range index.funcIDByKey {
		fn := &index.funcs[funcID]
		funcInfo.ID = formatFuncName("", key.TypeName, key.FuncName)
		funcInfo.PkgName = key.PkgName
		funcInfo.MaxHeatLevel = int(fn.maxLocalLevel)
		funcInfo.MaxGlobalHeatLevel = int(fn.maxGlobalLevel)
		funcInfo.Filename = index.filenames[fn.fileID]
		data := index.dataPoints[fn.dataFrom:fn.dataTo]
		for _, pt := range data {
			callback(LineStats{
				LineNum:         int(pt.line),
				Value:           pt.value,
				HeatLevel:       pt.flags.GetLocalLevel(),
				GlobalHeatLevel: pt.flags.GetGlobalLevel(),
				Func:            &funcInfo,
			})
		}
	}
}

// QueryLineRange scans the file data points that are located in [lineFrom, lineTo] range.
// callback is called for every matching data point.
// Returning false from the callback causes the iteration to stop early.
func (index *Index) QueryLineRange(key Key, lineFrom, lineTo int, callback func(stats LineStats) bool) {
	if lineFrom == lineTo {
		callback(index.QueryLine(key, lineFrom))
		return
	}
	index.queryLineRange(key, lineFrom, lineTo, callback)
}

func (index *Index) QueryLine(key Key, line int) LineStats {
	var result LineStats
	funcID, ok := index.funcIDByKey[key]
	if !ok {
		return result
	}
	fn := index.funcs[funcID]

	// A quick range check to avoid the search.
	if line < int(fn.minLine) || line > int(fn.maxLine) {
		return result
	}

	data := index.dataPoints[fn.dataFrom:fn.dataTo]
	if len(data) <= 4 {
		// Short data slice, use a linear search.
		for i := range data {
			pt := &data[i]
			if pt.line == uint32(line) {
				result = pt.Stats()
				break
			}
		}
	} else {
		// Use a binary search for bigger data slices.
		i := sort.Search(len(data), func(i int) bool {
			return data[i].line >= uint32(line)
		})
		if i < len(data) && data[i].line == uint32(line) {
			result = data[i].Stats()
		}
	}

	return result
}

func (index *Index) queryLineRange(key Key, lineFrom, lineTo int, callback func(stats LineStats) bool) {
	if lineFrom > lineTo {
		panic("lineFrom > lineTo")
	}

	funcID, ok := index.funcIDByKey[key]
	if !ok {
		return
	}
	fn := index.funcs[funcID]

	// A quick range check to avoid the search.
	if int(fn.maxLine) < lineFrom || int(fn.minLine) > lineTo {
		return
	}

	// Narrow the search window by the data points range.
	if int(fn.minLine) > lineFrom {
		lineFrom = int(fn.minLine)
	}
	if int(fn.maxLine) < lineTo {
		lineTo = int(fn.maxLine)
	}

	data := index.dataPoints[fn.dataFrom:fn.dataTo]

	// It's possible to optimize the case where f.minLine=lineFrom && f.maxLine=lineTo,
	// where we would just walk the entire data slice, but
	// that use case doesn't look compelling enough to add an extra branch to the code.
	i := sort.Search(len(data), func(i int) bool {
		return data[i].line >= uint32(lineFrom)
	})
	if i < len(data) && data[i].line >= uint32(lineFrom) && data[i].line <= uint32(lineTo) {
		pt := &data[i]
		// i is a first matching entry, the leftmost one.
		if !callback(pt.Stats()) {
			return
		}
		// All data points until lineTo are matched too.
		for j := i + 1; j < len(data) && data[j].line <= uint32(lineTo); j++ {
			pt := &data[j]
			if !callback(pt.Stats()) {
				return
			}
		}
	}
}
