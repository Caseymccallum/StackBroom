package netstat

import (
	"encoding/binary"
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"unsafe"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// Mirror structs exist only to ask the Go compiler for the Windows ABI offsets
// and strides. Microsoft warns that MIB_TCPTABLE_OWNER_PID may contain padding
// between dwNumEntries and the first row, so the parser derives the offset
// instead of assuming that a row starts at byte 4.
type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPID  uint32
}

type mibTCP6RowOwnerPID struct {
	LocalAddr     [16]byte
	LocalScopeID  uint32
	LocalPort     uint32
	RemoteAddr    [16]byte
	RemoteScopeID uint32
	RemotePort    uint32
	State         uint32
	OwningPID     uint32
}

type mibTCPTableOwnerPID struct {
	Count uint32
	Rows  [1]mibTCPRowOwnerPID
}

type mibTCP6TableOwnerPID struct {
	Count uint32
	Rows  [1]mibTCP6RowOwnerPID
}

var (
	ipv4FirstRow = int(unsafe.Offsetof(mibTCPTableOwnerPID{}.Rows))
	ipv4RowSize  = int(unsafe.Sizeof(mibTCPRowOwnerPID{}))
	ipv6FirstRow = int(unsafe.Offsetof(mibTCP6TableOwnerPID{}.Rows))
	ipv6RowSize  = int(unsafe.Sizeof(mibTCP6RowOwnerPID{}))
)

const mibTCPStateListen = 2

// ParseTable decodes a MIB_TCPTABLE_OWNER_PID or MIB_TCP6TABLE_OWNER_PID
// buffer. It validates the declared row count against the buffer length before
// reading any row and rejects absurd counts and oversized buffers.
//
// The buffer may be larger than count*rowSize: Windows sizes the allocation
// before filling it, so a table that shrank between calls leaves unused bytes
// at the end. A live probe observed 8 to 248 such bytes for IPv4 and up to
// 1,744 for IPv6. Trailing bytes are ignored; the declared count defines how
// many rows are read, and the total allocation is capped.
func ParseTable(data []byte, family string) ([]model.TCPListener, error) {
	if len(data) > maxTableBytes {
		return nil, fmt.Errorf("table buffer is %d bytes, above the %d-byte limit", len(data), maxTableBytes)
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("table buffer is %d bytes, shorter than the 4-byte row count", len(data))
	}

	count := binary.LittleEndian.Uint32(data[:4])
	if count > maxRows {
		return nil, fmt.Errorf("table declares %d rows, above the %d-row limit", count, maxRows)
	}

	first, stride, err := rowLayout(family)
	if err != nil {
		return nil, err
	}
	required := first + int(count)*stride
	if required > len(data) {
		return nil, fmt.Errorf("table declares %d rows requiring %d bytes, but only %d bytes were returned",
			count, required, len(data))
	}
	listeners := make([]model.TCPListener, 0, count)
	for i := 0; i < int(count); i++ {
		row := data[first+i*stride : first+(i+1)*stride]
		listener, ok := decodeRow(row, family)
		if ok {
			listeners = append(listeners, listener)
		}
	}
	return listeners, nil
}

func rowLayout(family string) (first, stride int, err error) {
	switch family {
	case FamilyIPv4:
		return ipv4FirstRow, ipv4RowSize, nil
	case FamilyIPv6:
		return ipv6FirstRow, ipv6RowSize, nil
	default:
		return 0, 0, fmt.Errorf("unknown address family %q", family)
	}
}

func decodeRow(row []byte, family string) (model.TCPListener, bool) {
	switch family {
	case FamilyIPv4:
		return decodeIPv4Row(row)
	case FamilyIPv6:
		return decodeIPv6Row(row)
	default:
		return model.TCPListener{}, false
	}
}

func decodeIPv4Row(row []byte) (model.TCPListener, bool) {
	if binary.LittleEndian.Uint32(row[0:4]) != mibTCPStateListen {
		return model.TCPListener{}, false
	}
	port := int(binary.BigEndian.Uint16(row[8:10]))
	pid := binary.LittleEndian.Uint32(row[20:24])
	if port == 0 || pid > math.MaxInt32 {
		return model.TCPListener{}, false
	}
	addr := netip.AddrFrom4([4]byte{row[4], row[5], row[6], row[7]})
	return model.TCPListener{
		Protocol:  Protocol,
		Family:    FamilyIPv4,
		Address:   addr.String(),
		Port:      port,
		PID:       int32(pid),
		Wildcard:  addr.IsUnspecified(),
		Ownership: model.Unknown,
	}, true
}

func decodeIPv6Row(row []byte) (model.TCPListener, bool) {
	if binary.LittleEndian.Uint32(row[48:52]) != mibTCPStateListen {
		return model.TCPListener{}, false
	}
	port := int(binary.BigEndian.Uint16(row[20:22]))
	pid := binary.LittleEndian.Uint32(row[52:56])
	if port == 0 || pid > math.MaxInt32 {
		return model.TCPListener{}, false
	}

	var raw [16]byte
	copy(raw[:], row[0:16])
	addr := netip.AddrFrom16(raw)
	if scope := binary.LittleEndian.Uint32(row[16:20]); scope != 0 && addr.Is6() && !addr.Is4In6() {
		addr = addr.WithZone(strconv.FormatUint(uint64(scope), 10))
	}
	return model.TCPListener{
		Protocol:  Protocol,
		Family:    FamilyIPv6,
		Address:   addr.String(),
		Port:      port,
		PID:       int32(pid),
		Wildcard:  addr.IsUnspecified(),
		Ownership: model.Unknown,
	}, true
}
