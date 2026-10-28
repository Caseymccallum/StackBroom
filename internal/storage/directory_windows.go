//go:build windows

package storage

import (
	"fmt"
	"os"
	"syscall"
)

const fileListDirectory = 0x00000001

// openVerifiedDirectory opens a directory without following a reparse point
// and verifies the attributes on the same handle used for enumeration.
func openVerifiedDirectory(path string) (*os.File, error) {
	pointer, err := syscall.UTF16PtrFromString(platformPath(path))
	if err != nil {
		return nil, fmt.Errorf("folder path is not usable")
	}
	handle, err := syscall.CreateFile(
		pointer,
		fileListDirectory|fileReadAttributes,
		fileShareRead|fileShareWrite|fileShareDelete,
		nil,
		openExisting,
		fileFlagBackupSemantics|fileFlagOpenReparsePoint,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("folder could not be opened")
	}
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil {
		_ = syscall.CloseHandle(handle)
		return nil, fmt.Errorf("folder identity could not be read")
	}
	if info.FileAttributes&fileAttributeReparsePoint != 0 {
		_ = syscall.CloseHandle(handle)
		return nil, fmt.Errorf("folder is a reparse point")
	}
	return os.NewFile(uintptr(handle), path), nil
}
