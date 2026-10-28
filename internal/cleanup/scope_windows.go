//go:build windows

package cleanup

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

// FOLDERID_LocalAppData {F1B32785-6FBA-4FCF-9D55-7B8E7F157091}
var localAppDataFolderID = struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}{0xF1B32785, 0x6FBA, 0x4FCF, [8]byte{0x9D, 0x55, 0x7B, 0x8E, 0x7F, 0x15, 0x70, 0x91}}

var (
	modShell32Cleanup = syscall.NewLazyDLL("shell32.dll")
	modOle32Cleanup   = syscall.NewLazyDLL("ole32.dll")
	procKnownFolder   = modShell32Cleanup.NewProc("SHGetKnownFolderPath")
	procCoTaskMemFree = modOle32Cleanup.NewProc("CoTaskMemFree")
)

func platformTrustedRoot(token string) (string, error) {
	var localAppData string
	switch token {
	case "%TEMP%", "%LOCALAPPDATA%":
		path, err := knownLocalAppData()
		if err != nil {
			return "", err
		}
		localAppData = path
	}
	switch token {
	case "%LOCALAPPDATA%":
		return localAppData, nil
	case "%TEMP%":
		return filepath.Join(localAppData, "Temp"), nil
	default:
		return "", fmt.Errorf("unsupported cleanup scope token")
	}
}

func utf16PointerToString(pointer *uint16) string {
	if pointer == nil {
		return ""
	}
	var units []uint16
	for offset := unsafe.Pointer(pointer); ; offset = unsafe.Add(offset, 2) {
		value := *(*uint16)(offset)
		if value == 0 {
			break
		}
		units = append(units, value)
	}
	return syscall.UTF16ToString(units)
}

func knownLocalAppData() (string, error) {
	var pathPointer *uint16
	result, _, _ := procKnownFolder.Call(
		uintptr(unsafe.Pointer(&localAppDataFolderID)),
		0,
		0,
		uintptr(unsafe.Pointer(&pathPointer)),
	)
	if result != 0 || pathPointer == nil {
		return "", fmt.Errorf("known folder lookup failed")
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(pathPointer)))
	path := utf16PointerToString(pathPointer)
	if path == "" {
		return "", fmt.Errorf("known folder returned no path")
	}
	return filepath.Clean(path), nil
}
