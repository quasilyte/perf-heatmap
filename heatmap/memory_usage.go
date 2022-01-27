package heatmap

func memoryUsageApprox(index *Index) int {
	size := 0

	// Size for keys.
	size += len(index.funcIDByKey) * (16 * 4)
	// Size for values.
	size += len(index.funcIDByKey) * 4

	size += cap(index.dataPoints) * 16
	size += cap(index.funcs) * 24

	size += cap(index.filenames) * 12
	for _, filename := range index.filenames {
		size += len(filename)
	}

	return size
}
