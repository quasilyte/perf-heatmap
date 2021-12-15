package heatmap

import (
	"sort"

	"github.com/google/pprof/profile"
)

// Index...
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
	// Threshold specifies where is the line between the "cold" and "hot" code.
	// If unset (0), the default value of 0.25 will be implied.
	//
	// The threshould value can be interpreted in this way: what percentage
	// of top sample records we're marking as hot. For 0.25 it's top 25% results.
	// For 0.5 we're taking the more significant half.
	// 1.0 includes all results.
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

// NewIndex creates an empty heatmap index.
// Use AddProfile method to populate it.
func NewIndex(config IndexConfig) *Index {
	if config.Threshold == 0 {
		config.Threshold = 0.25
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
}

func (index *Index) InspectFile(filename string, visit func(LineStats)) {
	f := index.byFilename[filename]
	data := index.dataPoints[f.dataFrom:f.dataTo]
	for _, pt := range data {
		visit(LineStats{
			LineNum:         int(pt.line),
			Value:           pt.value,
			HeatLevel:       pt.flags.GetLocalLevel(),
			GlobalHeatLevel: pt.flags.GetGlobalLevel(),
		})
	}
}
