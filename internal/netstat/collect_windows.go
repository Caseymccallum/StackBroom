//go:build windows

package netstat

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	iphlpapi                = syscall.NewLazyDLL("iphlpapi.dll")
	procGetExtendedTcpTable = iphlpapi.NewProc("GetExtendedTcpTable")
)

const (
	afInet                   = 2
	afInet6                  = 23
	tcpTableOwnerPIDListener = 3
	errorInsufficientBuffer  = 122
	tableFetchAttempts       = 3
)

func platformSupported() bool { return true }

func defaultTableSource() tableSource { return windowsTableSource{} }

// extendedTCPTable is a package variable so a Windows test can exercise the
// resize/retry policy without reading the live table.
var extendedTCPTable = func(buffer uintptr, size *uint32, af uint32) uint32 {
	r, _, _ := procGetExtendedTcpTable.Call(
		buffer,
		uintptr(unsafe.Pointer(size)),
		1,
		uintptr(af),
		uintptr(tcpTableOwnerPIDListener),
		0,
	)
	return uint32(r)
}

type windowsTableSource struct{}

// Table asks Windows for one listener table. The first call requests the
// required size; the second fills a bounded buffer. If the table grew between
// the calls Windows returns ERROR_INSUFFICIENT_BUFFER and the new size, and the
// fetch is retried a bounded number of times.
func (windowsTableSource) Table(family string) ([]byte, error) {
	af, ok := addressFamily(family)
	if !ok {
		return nil, fmt.Errorf("unknown address family %q", family)
	}

	var size uint32
	r := extendedTCPTable(0, &size, af)
	switch r {
	case 0:
		// No table was returned with a null buffer.
		return make([]byte, 4), nil
	case errorInsufficientBuffer:
		// size now holds the required size.
	default:
		return nil, &TableError{Family: family, Code: uint32(r)}
	}

	for attempt := 0; attempt < tableFetchAttempts; attempt++ {
		if size == 0 {
			return make([]byte, 4), nil
		}
		if uint64(size) > maxTableBytes {
			return nil, fmt.Errorf("Windows reported a %d-byte %s table, above the %d-byte limit",
				size, family, maxTableBytes)
		}
		buf := make([]byte, size)
		r = extendedTCPTable(uintptr(unsafe.Pointer(&buf[0])), &size, af)
		switch r {
		case 0:
			return buf, nil
		case errorInsufficientBuffer:
			// The table changed size; size has been updated for the retry.
			continue
		default:
			return nil, &TableError{Family: family, Code: uint32(r)}
		}
	}
	return nil, fmt.Errorf("the %s TCP listener table kept changing size after %d attempts",
		family, tableFetchAttempts)
}

func addressFamily(family string) (uint32, bool) {
	switch family {
	case FamilyIPv4:
		return afInet, true
	case FamilyIPv6:
		return afInet6, true
	default:
		return 0, false
	}
}
