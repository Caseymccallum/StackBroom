//go:build !windows

package storage

import (
	"fmt"
	"os"
)

func openVerifiedDirectory(path string) (*os.File, error) {
	handle, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("folder could not be opened")
	}
	info, err := handle.Stat()
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		handle.Close()
		return nil, fmt.Errorf("folder is not a normal directory")
	}
	return handle, nil
}
