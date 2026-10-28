//go:build windows

package storage

import (
	"syscall"
	"testing"
)

func setHiddenAttribute(t *testing.T, path string) {
	t.Helper()
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(pointer, syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
		t.Fatal(err)
	}
}
