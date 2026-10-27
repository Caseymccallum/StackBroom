package netstat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeTableSource struct {
	tables map[string][]byte
	errs   map[string]error
}

func (f fakeTableSource) Table(family string) ([]byte, error) {
	if err := f.errs[family]; err != nil {
		return nil, err
	}
	return f.tables[family], nil
}

func TestCollectFromPartialFamilyFailure(t *testing.T) {
	fixed := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	source := fakeTableSource{
		tables: map[string][]byte{
			FamilyIPv4: tableBytes(ipv4RowBytes(2, [4]byte{127, 0, 0, 1}, 5173, 100)),
		},
		errs: map[string]error{
			FamilyIPv6: &TableError{Family: FamilyIPv6, Code: 50},
		},
	}
	got := collectFrom(source, func() time.Time { return fixed })
	if !got.Available || !got.IPv4 || got.IPv6 {
		t.Fatalf("availability = %+v", got)
	}
	if len(got.Listeners) != 1 || got.Listeners[0].Port != 5173 {
		t.Errorf("listeners = %+v", got.Listeners)
	}
	if got.SampledAt != fixed {
		t.Errorf("sampledAt = %s", got.SampledAt)
	}
	if got.Reason != "" {
		t.Errorf("reason = %q, want empty when one family succeeded", got.Reason)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "not supported") {
		t.Errorf("warnings = %v", got.Warnings)
	}
}

func TestCollectFromBothFamiliesFail(t *testing.T) {
	source := fakeTableSource{errs: map[string]error{
		FamilyIPv4: &TableError{Family: FamilyIPv4, Code: 5},
		FamilyIPv6: &TableError{Family: FamilyIPv6, Code: 5},
	}}
	got := collectFrom(source, time.Now)
	if got.Available || got.IPv4 || got.IPv6 {
		t.Fatalf("availability = %+v", got)
	}
	if !strings.Contains(got.Reason, "access denied") {
		t.Errorf("reason = %q", got.Reason)
	}
	if got.Listeners == nil || len(got.Listeners) != 0 {
		t.Errorf("listeners = %+v, want a non-nil empty slice", got.Listeners)
	}
	if len(got.Warnings) != 2 {
		t.Errorf("warnings = %v", got.Warnings)
	}
}

func TestCollectFromParseFailureKeepsOtherFamily(t *testing.T) {
	source := fakeTableSource{tables: map[string][]byte{
		FamilyIPv4: []byte{1, 2, 3},
		FamilyIPv6: tableBytes(),
	}}
	got := collectFrom(source, time.Now)
	if !got.Available || got.IPv4 || !got.IPv6 {
		t.Fatalf("availability = %+v", got)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "rejected") {
		t.Errorf("warnings = %v", got.Warnings)
	}
}

func TestCollectFromSortsListeners(t *testing.T) {
	source := fakeTableSource{tables: map[string][]byte{
		FamilyIPv4: tableBytes(
			ipv4RowBytes(2, [4]byte{0, 0, 0, 0}, 9000, 9),
			ipv4RowBytes(2, [4]byte{127, 0, 0, 1}, 5173, 7),
		),
		FamilyIPv6: tableBytes(
			ipv6RowBytes(2, [16]byte{15: 1}, 0, 3000, 3),
		),
	}}
	got := collectFrom(source, time.Now)
	if len(got.Listeners) != 3 {
		t.Fatalf("listeners = %+v", got.Listeners)
	}
	want := []int{3000, 5173, 9000}
	for i, port := range want {
		if got.Listeners[i].Port != port {
			t.Errorf("listener %d port = %d, want %d", i, got.Listeners[i].Port, port)
		}
	}
}

func TestCollectFromZeroListeners(t *testing.T) {
	source := fakeTableSource{tables: map[string][]byte{
		FamilyIPv4: tableBytes(),
		FamilyIPv6: tableBytes(),
	}}
	got := collectFrom(source, time.Now)
	if !got.Available || !got.IPv4 || !got.IPv6 {
		t.Fatalf("availability = %+v", got)
	}
	if len(got.Listeners) != 0 || len(got.Warnings) != 0 {
		t.Errorf("result = %+v", got)
	}
}

func TestDescribeTableError(t *testing.T) {
	tests := []struct {
		code uint32
		want string
	}{
		{5, "access denied"},
		{50, "not supported"},
		{87, "invalid"},
		{122, "changing size"},
		{1234, "Windows error 1234"},
	}
	for _, tc := range tests {
		got := describeTableError(FamilyIPv4, tc.code)
		if !strings.Contains(got, tc.want) {
			t.Errorf("code %d = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestCollectorHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := Collector{source: fakeTableSource{}}.Collect(ctx)
	if got.Available || !strings.Contains(got.Reason, "cancelled") {
		t.Fatalf("result = %+v", got)
	}
	if got.Listeners == nil {
		t.Error("listeners should be an empty slice, not nil")
	}
}

func TestCollectFromGenericError(t *testing.T) {
	source := fakeTableSource{errs: map[string]error{
		FamilyIPv4: errors.New("syscall failed"),
		FamilyIPv6: errors.New("syscall failed"),
	}}
	got := collectFrom(source, time.Now)
	if got.Available || !strings.Contains(got.Reason, "syscall failed") {
		t.Fatalf("result = %+v", got)
	}
}
