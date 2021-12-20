package heatmap

func formatFuncName(pkgName, typeName, funcName string) string {
	if typeName != "" {
		if pkgName != "" {
			return pkgName + ".(" + typeName + ")." + funcName
		}
		return "(" + typeName + ")." + funcName
	}
	if pkgName != "" {
		return pkgName + "." + funcName
	}
	return funcName
}
