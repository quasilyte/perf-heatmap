package heatmap

import (
	"math"
)

const maxHeatLevel = 5

type fileIndex struct {
	minLine uint32
	maxLine uint32

	dataFrom int
	dataTo   int
}

func (f *fileIndex) NumPoints() int {
	return f.dataTo - f.dataFrom
}

// dataPoint is a compact index data unit.
//
// To fit it into two 64-bit words, we store line numbers
// as uint32 instead of int64. Having a line number that doesn't
// fit that is unlikely (4_294_967_295 lines), but even in that
// case we'll just skip samples that go beyond that.
// The saved 32 bits go into the extra metadata. See dataPointFlags.
type dataPoint struct {
	line  uint32
	flags dataPointFlags
	value int64
}

// Upper 3 bits are for the local level value.
// Next 3 bits are for the global level value.
// Other lower bits are bit flags.
type dataPointFlags uint32

func (flags *dataPointFlags) GetLocalLevel() int {
	const mask = (0b111 << (32 - 3))
	return int(*flags&mask) >> (32 - 3)
}

func (flags *dataPointFlags) GetGlobalLevel() int {
	const mask = (0b111 << (32 - 6))
	return int(*flags&mask) >> (32 - 6)
}

func (flags *dataPointFlags) SetLocalLevel(level int) {
	if level < 0 || level > maxHeatLevel {
		panic("invalid level value") // Should never happen.
	}
	const mask = (0b111 << (32 - 3))
	*(*uint32)(flags) &^= mask
	*(*uint32)(flags) |= uint32(level) << (32 - 3)
}

func (flags *dataPointFlags) SetGlobalLevel(level int) {
	if level < 0 || level > maxHeatLevel {
		panic("invalid level value") // Should never happen.
	}
	const mask = (0b111 << (32 - 6))
	*(*uint32)(flags) &^= mask
	*(*uint32)(flags) |= uint32(level) << (32 - 6)
}

func forChunks(length, n int, visit func(chunkNum, i int)) {
	if length < n {
		n = length
	}
	avgLen := int(math.Round(float64(length) / float64(n)))
	delta := length - (avgLen * n)
	rem := delta

	i := 0
	for chunkNum := 0; chunkNum < n; chunkNum++ {
		// First chunk will include the remainder (which can be negative).
		numElems := avgLen
		if chunkNum == 0 {
			numElems += rem
		}
		for j := 0; j < numElems; j++ {
			visit(chunkNum, i)
			i++
		}
	}
}
