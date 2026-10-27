//go:build windows

package netstat

import (
	"strings"
	"testing"
)

// overrideTable swaps the API call for a fake and restores it after the test.
func overrideTable(t *testing.T, fake func(buffer uintptr, size *uint32, af uint32) uint32) {
	t.Helper()
	previous := extendedTCPTable
	extendedTCPTable = fake
	t.Cleanup(func() { extendedTCPTable = previous })
}

func TestWindowsSourceRetriesAfterSizeChange(t *testing.T) {
	calls := 0
	overrideTable(t, func(buffer uintptr, size *uint32, af uint32) uint32 {
		if af != afInet {
			t.Errorf("address family = %d, want %d", af, afInet)
		}
		calls++
		switch calls {
		case 1:
			if buffer != 0 {
				t.Error("size query should pass a null buffer")
			}
			*size = 64
			return errorInsufficientBuffer
		case 2:
			if buffer == 0 || *size != 64 {
				t.Errorf("first fill used buffer=%d size=%d", buffer, *size)
			}
			*size = 128
			return errorInsufficientBuffer
		case 3:
			if buffer == 0 || *size != 128 {
				t.Errorf("second fill used buffer=%d size=%d", buffer, *size)
			}
			return 0
		default:
			t.Fatalf("unexpected extra call %d", calls)
			return 0
		}
	})

	data, err := (windowsTableSource{}).Table(FamilyIPv4)
	if err != nil {
		t.Fatalf("Table: %v", err)
	}
	if len(data) != 128 {
		t.Errorf("buffer length = %d, want 128", len(data))
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestWindowsSourceStopsAfterRepeatedGrowth(t *testing.T) {
	calls := 0
	overrideTable(t, func(buffer uintptr, size *uint32, af uint32) uint32 {
		calls++
		*size = 64
		return errorInsufficientBuffer
	})
	_, err := (windowsTableSource{}).Table(FamilyIPv4)
	if err == nil || !strings.Contains(err.Error(), "kept changing size") {
		t.Fatalf("error = %v", err)
	}
	if calls != 1+tableFetchAttempts {
		t.Errorf("calls = %d, want %d", calls, 1+tableFetchAttempts)
	}
}

func TestWindowsSourceRejectsOversizedAllocation(t *testing.T) {
	calls := 0
	overrideTable(t, func(buffer uintptr, size *uint32, af uint32) uint32 {
		calls++
		*size = maxTableBytes + 1
		return errorInsufficientBuffer
	})
	_, err := (windowsTableSource{}).Table(FamilyIPv4)
	if err == nil || !strings.Contains(err.Error(), "above the") {
		t.Fatalf("error = %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want only the size query", calls)
	}
}

func TestWindowsSourceClassifiesAccessDenied(t *testing.T) {
	overrideTable(t, func(buffer uintptr, size *uint32, af uint32) uint32 {
		return 5
	})
	_, err := (windowsTableSource{}).Table(FamilyIPv6)
	tableErr, ok := err.(*TableError)
	if !ok {
		t.Fatalf("error = %T %v, want *TableError", err, err)
	}
	if tableErr.Family != FamilyIPv6 || tableErr.Code != 5 {
		t.Errorf("error = %+v", tableErr)
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("message = %q", err.Error())
	}
}
