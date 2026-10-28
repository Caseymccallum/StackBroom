//go:build !windows

package storage

import (
	"fmt"
	"os"
)

func openVerifiedContent(path string, expected *FileRecord) (*os.File, error) {
	if !pathWithinRoot(expected.Root, path) || !resolvedPathWithinRoot(expected.Root, path) {
		return nil, fmt.Errorf("target is outside the approved root or has a reparse ancestor")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("file is no longer available")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("target is no longer a regular file")
	}
	if uint64(info.Size()) != expected.LogicalBytes || !info.ModTime().Equal(expected.ModTime) {
		return nil, fmt.Errorf("target metadata changed before reading")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("target could not be opened for verified reading")
	}
	current, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("target identity could not be read")
	}
	if uint64(current.Size()) != expected.LogicalBytes || !current.ModTime().Equal(expected.ModTime) {
		file.Close()
		return nil, fmt.Errorf("target changed while opening")
	}
	return file, nil
}
