//go:build windows

package storage

import (
	"fmt"
	"os"
	"syscall"
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
	fileReadAttributes        = 0x00000080
	fileShareRead             = 0x00000001
	fileShareWrite            = 0x00000002
	fileShareDelete           = 0x00000004
	openExisting              = 3
	fileFlagBackupSemantics   = 0x02000000
	fileFlagOpenReparsePoint  = 0x00200000
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
