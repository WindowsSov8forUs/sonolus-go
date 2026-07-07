package utils

import "runtime/debug"

func SonolusPkgPath() string {
	info, _ := debug.ReadBuildInfo()
	return info.Path
}
