package heatmap

import "fmt"

const maxHeatLevel = 5

type fileIndex struct {
	minLine uint32
	maxLine uint32

	dataFrom int
	dataTo   int

	// Sorted by name.
	funcs []funcDataPoint
}

func (f *fileIndex) NumPoints() int {
	return f.dataTo - f.dataFrom
}

type funcDataPoint struct {
	id uint16

	maxLocalLevel  uint8
	maxGlobalLevel uint8

	name string
}

func (pt funcDataPoint) String() string {
	return fmt.Sprintf("{%d, %s}", pt.id, pt.name)
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
	line      uint32
	funcIndex uint16
	flags     dataPointFlags
	value     int64
}

// Upper 3 bits are for the local level value.
// Next 3 bits are for the global level value.
// Other (10) lower bits are bit flags.
type dataPointFlags uint16

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
