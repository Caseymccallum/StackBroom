//go:build windows

package storage

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	fileAttributeReadonly     = 0x00000001
	fileAttributeHidden       = 0x00000002
	fileAttributeSystem       = 0x00000004
	fileAttributeDirectory    = 0x00000010
	fileAttributeReparsePoint = 0x00000400
	fileAttributeSparseFile   = 0x00000200
	fileAttributeOffline      = 0x00001000
	fileAttributeRecallOnOpen = 0x00040000
	fileAttributeRecallOnData = 0x00400000
	fileReadData              = 0x00000001
	fileReadAttributes        = 0x00000080
	fileShareRead             = 0x00000001
	fileShareWrite            = 0x00000002
	fileShareDelete           = 0x00000004
	openExisting              = 3
	fileFlagBackupSemantics   = 0x02000000
	fileFlagOpenReparsePoint  = 0x00200000
	fileFlagSequentialScan    = 0x08000000
)

var (
	modKernel32Storage = syscall.NewLazyDLL("kernel32.dll")
	procGetCompressed  = modKernel32Storage.NewProc("GetCompressedFileSizeW")
)

// inspectFile reads metadata without opening file content. Cloud placeholders
// are identified before any content API is called.
func inspectFile(path string, info os.FileInfo) fileMeta {
	meta := fileMeta{}
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		meta.Hidden = data.FileAttributes&fileAttributeHidden != 0
		meta.System = data.FileAttributes&fileAttributeSystem != 0
		meta.Reparse = data.FileAttributes&fileAttributeReparsePoint != 0
		meta.Sparse = data.FileAttributes&fileAttributeSparseFile != 0
		meta.Placeholder = data.FileAttributes&(fileAttributeOffline|fileAttributeRecallOnOpen|fileAttributeRecallOnData) != 0
	}

	handle, err := openMetadataHandle(path)
	if err != nil {
		return meta
	}
	defer syscall.CloseHandle(handle)

	var byHandle syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &byHandle); err == nil {
		meta.LinkCount = byHandle.NumberOfLinks
		meta.FileID = fmt.Sprintf("%08x:%08x%08x", byHandle.VolumeSerialNumber, byHandle.FileIndexHigh, byHandle.FileIndexLow)
		if meta.Reparse || meta.Placeholder || info.IsDir() {
			meta.AllocatedKnown = false
		}
	}

	if !info.IsDir() && !meta.Placeholder {
		if allocated, ok := compressedSize(path); ok {
			meta.AllocatedBytes = allocated
			meta.AllocatedKnown = true
		}
	}
	return meta
}

func openMetadataHandle(path string) (syscall.Handle, error) {
	pathPointer, err := syscall.UTF16PtrFromString(platformPath(path))
	if err != nil {
		return syscall.InvalidHandle, err
	}
	handle, err := syscall.CreateFile(
		pathPointer,
		fileReadAttributes,
		fileShareRead|fileShareWrite|fileShareDelete,
		nil,
		openExisting,
		fileFlagBackupSemantics|fileFlagOpenReparsePoint,
		0,
	)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	return handle, nil
}

// openVerifiedContent opens the file with OPEN_REPARSE_POINT, checks its
// attributes and identity on that handle, and only then returns a reader.
// Placeholders and reparse points are rejected before any content is read.
func openVerifiedContent(path string, expected *FileRecord) (*os.File, error) {
	if !pathWithinRoot(expected.Root, path) || !resolvedPathWithinRoot(expected.Root, path) {
		return nil, fmt.Errorf("target is outside the approved root or has a reparse ancestor")
	}
	before, err := os.Lstat(platformPath(path))
	if err != nil {
		return nil, fmt.Errorf("file is no longer available")
	}
	if before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("target is no longer a regular file")
	}
	metaBefore := inspectFile(path, before)
	if metaBefore.Reparse || metaBefore.Placeholder {
		return nil, fmt.Errorf("target became a reparse point or cloud placeholder")
	}
	if uint64(before.Size()) != expected.LogicalBytes || !before.ModTime().Equal(expected.ModTime) {
		return nil, fmt.Errorf("target metadata changed before reading")
	}

	pathPointer, err := syscall.UTF16PtrFromString(platformPath(path))
	if err != nil {
		return nil, fmt.Errorf("target path is not usable")
	}
	handle, err := syscall.CreateFile(
		pathPointer,
		fileReadData|fileReadAttributes,
		fileShareRead|fileShareWrite|fileShareDelete,
		nil,
		openExisting,
		fileFlagOpenReparsePoint|fileFlagSequentialScan,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("target could not be opened for verified reading")
	}
	closeOnError := func() { _ = syscall.CloseHandle(handle) }

	var byHandle syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &byHandle); err != nil {
		closeOnError()
		return nil, fmt.Errorf("target identity could not be read")
	}
	attributes := byHandle.FileAttributes
	if attributes&(fileAttributeReparsePoint|fileAttributeOffline|fileAttributeRecallOnOpen|fileAttributeRecallOnData) != 0 {
		closeOnError()
		return nil, fmt.Errorf("target is a reparse point or cloud placeholder")
	}
	size := uint64(byHandle.FileSizeHigh)<<32 | uint64(byHandle.FileSizeLow)
	if size != expected.LogicalBytes {
		closeOnError()
		return nil, fmt.Errorf("target size changed before reading")
	}
	lastWrite := time.Unix(0, byHandle.LastWriteTime.Nanoseconds())
	if !lastWrite.Equal(expected.ModTime) {
		closeOnError()
		return nil, fmt.Errorf("target write time changed before reading")
	}
	fileID := fmt.Sprintf("%08x:%08x%08x", byHandle.VolumeSerialNumber, byHandle.FileIndexHigh, byHandle.FileIndexLow)
	if expected.FileID != "" && fileID != expected.FileID {
		closeOnError()
		return nil, fmt.Errorf("target identity changed before reading")
	}
	return os.NewFile(uintptr(handle), path), nil
}

func compressedSize(path string) (uint64, bool) {
	pathPointer, err := syscall.UTF16PtrFromString(platformPath(path))
	if err != nil {
		return 0, false
	}
	var high uint32
	low, _, callErr := procGetCompressed.Call(
		uintptr(unsafe.Pointer(pathPointer)),
		uintptr(unsafe.Pointer(&high)),
	)
	if uint32(low) == 0xFFFFFFFF && callErr != nil {
		return 0, false
	}
	return uint64(high)<<32 | uint64(uint32(low)), true
}
