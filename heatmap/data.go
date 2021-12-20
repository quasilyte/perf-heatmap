package heatmap

import (
	"fmt"
)

const maxHeatLevel = 5

type funcIndex struct {
	maxLocalLevel  uint8
	maxGlobalLevel uint8

	// Line ranges inside a containing file.
	minLine uint32
	maxLine uint32

	// Used to get the func data points range from the Index.
	dataFrom uint32
	dataTo   uint32

	fileID uint32
}

func (fn *funcIndex) NumPoints() int {
	return int(fn.dataTo - fn.dataFrom)
}

// dataPoint is a compact index data unit.
//
// To fit it into two 64-bit words, we store line numbers
// as uint32 instead of int64. Having a line number that doesn't
// fit that is unlikely (4_294_967_295 lines), but even in that
// case we'll just skip samples that go beyond that.
//
// The saved 32 bits go into the extra metadata.
// 16 bits are used to reference the associated function.
// fileIndex.funcs[dataPoint.funcIndex] correspond to a function info.
// If file contains more than 65535 functions with samples, oh well.
// Other 16 bits are occupied by dataPointFlags.
type dataPoint struct {
	line  uint32
	flags dataPointFlags
	value int64
}

func (pt *dataPoint) HeatLevel() HeatLevel {
	return HeatLevel{
		Local:  pt.flags.GetLocalLevel(),
		Global: pt.flags.GetGlobalLevel(),
	}
}

func (pt dataPoint) String() string {
	return fmt.Sprintf("{%d %s}", pt.value, pt.flags)
}

// Upper 3 bits are for the local level value.
// Next 3 bits are for the global level value.
// Other (10) lower bits are bit flags.
type dataPointFlags uint16

func (flags dataPointFlags) String() string {
	return fmt.Sprintf("<local=%d global=%d>",
		flags.GetLocalLevel(), flags.GetGlobalLevel())
}

func (flags *dataPointFlags) GetLocalLevel() int {
	const mask = (0b111 << (16 - 3))
	return int(*flags&mask) >> (16 - 3)
}

func (flags *dataPointFlags) GetGlobalLevel() int {
	const mask = (0b111 << (16 - 6))
	return int(*flags&mask) >> (16 - 6)
}

func (flags *dataPointFlags) SetLocalLevel(level int) {
	if level < 0 || level > maxHeatLevel {
		panic("invalid level value") // Should never happen.
	}
	const mask = (0b111 << (16 - 3))
	*(*uint16)(flags) &^= mask
	*(*uint16)(flags) |= uint16(level) << (16 - 3)
}

func (flags *dataPointFlags) SetGlobalLevel(level int) {
	if level < 0 || level > maxHeatLevel {
		panic("invalid level value") // Should never happen.
	}
	const mask = (0b111 << (16 - 6))
	*(*uint16)(flags) &^= mask
	*(*uint16)(flags) |= uint16(level) << (16 - 6)
}
