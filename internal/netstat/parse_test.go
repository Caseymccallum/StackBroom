package netstat

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func tableBytes(rows ...[]byte) []byte {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, uint32(len(rows)))
	for _, row := range rows {
		data = append(data, row...)
	}
	// Windows returned eight trailing alignment bytes for both families in a
	// live probe. Fixtures keep them so the parser is tested against reality.
	return append(data, make([]byte, 8)...)
}

func ipv4RowBytes(state uint32, address [4]byte, port int, pid uint32) []byte {
	row := make([]byte, ipv4RowSize)
	binary.LittleEndian.PutUint32(row[0:4], state)
	copy(row[4:8], address[:])
	binary.BigEndian.PutUint16(row[8:10], uint16(port))
	binary.LittleEndian.PutUint32(row[20:24], pid)
	return row
}

func ipv6RowBytes(state uint32, address [16]byte, scope uint32, port int, pid uint32) []byte {
	row := make([]byte, ipv6RowSize)
	copy(row[0:16], address[:])
	binary.LittleEndian.PutUint32(row[16:20], scope)
	binary.BigEndian.PutUint16(row[20:22], uint16(port))
	binary.LittleEndian.PutUint32(row[48:52], state)
	binary.LittleEndian.PutUint32(row[52:56], pid)
	return row
}

func TestParseIPv4Table(t *testing.T) {
	data := tableBytes(
		ipv4RowBytes(2, [4]byte{127, 0, 0, 1}, 3000, 1234),
		ipv4RowBytes(2, [4]byte{0, 0, 0, 0}, 8000, 4321),
	)
	got, err := ParseTable(data, FamilyIPv4)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("listeners = %+v", got)
	}
	if got[0].Address != "127.0.0.1" || got[0].Port != 3000 || got[0].PID != 1234 || got[0].Wildcard {
		t.Errorf("loopback listener = %+v", got[0])
	}
	if got[0].Protocol != "tcp" || got[0].Family != FamilyIPv4 || got[0].Ownership != model.Unknown {
		t.Errorf("listener metadata = %+v", got[0])
	}
	if got[1].Address != "0.0.0.0" || !got[1].Wildcard || got[1].PID != 4321 {
		t.Errorf("wildcard listener = %+v", got[1])
	}
}

func TestParseIPv4PortByteOrder(t *testing.T) {
	data := tableBytes(ipv4RowBytes(2, [4]byte{127, 0, 0, 1}, 5173, 1))
	row := data[ipv4FirstRow : ipv4FirstRow+ipv4RowSize]
	if row[8] != 0x14 || row[9] != 0x35 {
		t.Fatalf("fixture does not encode 5173 in network order: % x", row[8:10])
	}
	got, err := ParseTable(data, FamilyIPv4)
	if err != nil || len(got) != 1 {
		t.Fatalf("ParseTable = %+v, %v", got, err)
	}
	if got[0].Port != 5173 {
		t.Errorf("port = %d, want 5173", got[0].Port)
	}
}

func TestParseIPv6Table(t *testing.T) {
	loopback := [16]byte{15: 1}
	linkLocal := [16]byte{0: 0xfe, 1: 0x80, 15: 1}
	data := tableBytes(
		ipv6RowBytes(2, loopback, 0, 5173, 111),
		ipv6RowBytes(2, [16]byte{}, 0, 80, 4),
		ipv6RowBytes(2, linkLocal, 12, 8080, 222),
	)
	got, err := ParseTable(data, FamilyIPv6)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("listeners = %+v", got)
	}
	if got[0].Address != "::1" || got[0].Port != 5173 || got[0].PID != 111 || got[0].Wildcard {
		t.Errorf("loopback = %+v", got[0])
	}
	if got[1].Address != "::" || !got[1].Wildcard {
		t.Errorf("wildcard = %+v", got[1])
	}
	if got[2].Address != "fe80::1%12" {
		t.Errorf("scoped link-local address = %q", got[2].Address)
	}
}

func TestParseFiltersNonListeningRows(t *testing.T) {
	data := tableBytes(
		ipv4RowBytes(5, [4]byte{127, 0, 0, 1}, 3000, 1), // established
		ipv4RowBytes(2, [4]byte{127, 0, 0, 1}, 0, 2),    // port zero
		ipv4RowBytes(2, [4]byte{127, 0, 0, 1}, 3001, 0xffffffff),
		ipv4RowBytes(2, [4]byte{127, 0, 0, 1}, 3002, 42),
	)
	got, err := ParseTable(data, FamilyIPv4)
	if err != nil {
		t.Fatalf("ParseTable: %v", err)
	}
	if len(got) != 1 || got[0].Port != 3002 || got[0].PID != 42 {
		t.Fatalf("listeners = %+v", got)
	}
}

func TestParseTableRejectsMalformedInput(t *testing.T) {
	tooMany := make([]byte, 4)
	binary.LittleEndian.PutUint32(tooMany, maxRows+1)
	tests := []struct {
		name   string
		data   []byte
		family string
		want   string
	}{
		{"empty", nil, FamilyIPv4, "shorter"},
		{"short header", []byte{1, 2, 3}, FamilyIPv4, "shorter"},
		{"unknown family", tableBytes(), "ipx", "unknown address family"},
		{"row count too large", tooMany, FamilyIPv4, "above the"},
		{"truncated rows", func() []byte {
			data := make([]byte, 4)
			binary.LittleEndian.PutUint32(data, 2)
			return append(data, make([]byte, ipv4RowSize)...)
		}(), FamilyIPv4, "only"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTable(tc.data, tc.family)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestParseTableAcceptsEmptyAndTrailingAllocationSlack(t *testing.T) {
	for _, data := range [][]byte{
		make([]byte, 4),
		append(make([]byte, 4), make([]byte, 2048)...),
	} {
		got, err := ParseTable(data, FamilyIPv4)
		if err != nil || len(got) != 0 {
			t.Fatalf("ParseTable(%d bytes) = %+v, %v", len(data), got, err)
		}
	}

	// A shrunken table can leave far more slack than a row. Only the declared
	// count is read, so the trailing bytes must not become listeners.
	data := append(tableBytes(ipv4RowBytes(2, [4]byte{127, 0, 0, 1}, 5173, 7)), make([]byte, 2048)...)
	got, err := ParseTable(data, FamilyIPv4)
	if err != nil || len(got) != 1 || got[0].Port != 5173 {
		t.Fatalf("ParseTable with slack = %+v, %v", got, err)
	}
}
