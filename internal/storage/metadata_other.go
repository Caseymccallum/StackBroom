//go:build !windows

package storage

import "os"

// inspectFile provides the same shape on non-Windows development hosts.
func inspectFile(path string, info os.FileInfo) fileMeta {
	return fileMeta{
		Reparse:        info.Mode()&os.ModeSymlink != 0,
		AllocatedBytes: uint64(maxInt64(info.Size())),
		AllocatedKnown: true,
	}
}

func maxInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
