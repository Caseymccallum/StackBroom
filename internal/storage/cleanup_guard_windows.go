//go:build windows

package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
)

// GuardCleanupAncestors pins every directory from the volume root to the
// target's parent without write/delete sharing. Failures release all handles.
// This narrows ancestor substitution; Shell still binds the final file by path.
func GuardCleanupAncestors(file FileRecord) (func(), error) {
	var handles []syscall.Handle
	closeAll := func() {
		for i := len(handles) - 1; i >= 0; i-- {
			_ = syscall.CloseHandle(handles[i])
		}
	}
	fail := func() (func(), error) { closeAll(); return nil, fmt.Errorf("ancestor guard unavailable") }
	path := filepath.Clean(file.Path)
	volume := filepath.VolumeName(path)
	if len(volume) != 2 || volume[1] != ':' || !filepath.IsAbs(path) {
		return fail()
	}
	parent := filepath.Dir(path)
	current := volume + string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(parent, current), string(filepath.Separator))
	directories := []string{current}
	for _, part := range parts {
		if part != "" {
			current = filepath.Join(current, part)
			directories = append(directories, current)
		}
	}
	for _, directory := range directories {
		ptr, err := syscall.UTF16PtrFromString(platformPath(directory))
		if err != nil {
			return fail()
		}
		// FILE_LIST_DIRECTORY (same bit as FILE_READ_DATA) makes this a
		// sharing-enforced open. Attribute-only directory opens do not provide
		// a reliable rename/delete exclusion on Windows.
		handle, err := syscall.CreateFile(ptr, fileReadData|fileReadAttributes, fileShareRead, nil, openExisting, fileFlagBackupSemantics|fileFlagOpenReparsePoint, 0)
		if err != nil {
			return fail()
		}
		handles = append(handles, handle)
		var info syscall.ByHandleFileInformation
		if syscall.GetFileInformationByHandle(handle, &info) != nil || info.FileAttributes&fileAttributeDirectory == 0 || info.FileAttributes&(fileAttributeReparsePoint|fileAttributeOffline|fileAttributeRecallOnOpen|fileAttributeRecallOnData) != 0 {
			return fail()
		}
	}
	if ValidateCleanupTarget(&file) != nil {
		return fail()
	}
	return closeAll, nil
}
