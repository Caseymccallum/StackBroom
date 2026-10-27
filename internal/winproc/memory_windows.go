//go:build windows

package winproc

import "unsafe"

var procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// HostMemory returns total and currently available physical memory.
//
// Available is Windows' own estimate of memory that can be given to processes
// without causing paging. It is not "free" memory in the strict sense.
func HostMemory() (total uint64, available uint64, ok bool) {
	var status memoryStatusEx
	status.Length = uint32(unsafe.Sizeof(status))
	r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if r == 0 {
		return 0, 0, false
	}
	return status.TotalPhys, status.AvailPhys, true
}
