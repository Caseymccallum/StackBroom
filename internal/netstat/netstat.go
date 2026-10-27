// Package netstat observes listening TCP sockets on Windows, read-only.
//
// The Windows syscall lives in collect_windows.go. Binary row parsing and the
// availability policy live in platform-neutral files so fixtures can exercise
// every layout without contacting the host.
package netstat

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// Address families reported by the collector.
const (
	FamilyIPv4 = "ipv4"
	FamilyIPv6 = "ipv6"
	Protocol   = "tcp"
)

// Bounds protect against a corrupted or hostile table. A table cannot contain
// more rows than this without being rejected before allocation of per-row
// records.
const (
	maxRows       = 200000
	maxTableBytes = 64 << 20
)

// TableError describes a Windows API failure without guessing at its cause.
type TableError struct {
	Family string
	Code   uint32
}

func (e *TableError) Error() string {
	return describeTableError(e.Family, e.Code)
}

// Collector reads the IPv4 and IPv6 listener tables. It creates no socket and
// sends no packet.
type Collector struct {
	// Now supplies the sample timestamp; nil uses time.Now.
	Now func() time.Time
	// source is an injection point for fixture-driven tests.
	source tableSource
}

// tableSource returns the raw table buffer for one address family.
type tableSource interface {
	Table(family string) ([]byte, error)
}

// Collect reads both address families. A failure in one family does not hide a
// successful result from the other; the distinction is explicit in the result.
func (c Collector) Collect(ctx context.Context) model.NetworkInfo {
	if err := ctx.Err(); err != nil {
		return model.NetworkInfo{
			Available: false,
			Reason:    "TCP listener inspection was cancelled.",
			Listeners: []model.TCPListener{},
		}
	}
	now := c.Now
	if now == nil {
		now = time.Now
	}
	if !platformSupported() {
		return model.NetworkInfo{
			Available: false,
			Reason:    "TCP listener inspection is only available on Windows.",
			Listeners: []model.TCPListener{},
			SampledAt: now(),
		}
	}
	source := c.source
	if source == nil {
		source = defaultTableSource()
	}
	return collectFrom(source, now)
}

func collectFrom(source tableSource, now func() time.Time) model.NetworkInfo {
	info := model.NetworkInfo{
		Listeners: []model.TCPListener{},
		SampledAt: now(),
	}
	var failures []string

	for _, family := range []string{FamilyIPv4, FamilyIPv6} {
		data, err := source.Table(family)
		if err != nil {
			reason := describeFamilyError(family, err)
			failures = append(failures, family+": "+reason)
			info.Warnings = append(info.Warnings,
				fmt.Sprintf("%s TCP listeners could not be read: %s.", strings.ToUpper(family), reason))
			continue
		}
		listeners, err := ParseTable(data, family)
		if err != nil {
			reason := "the listener table was rejected: " + err.Error()
			failures = append(failures, family+": "+reason)
			info.Warnings = append(info.Warnings,
				fmt.Sprintf("%s TCP listener table was rejected: %s.", strings.ToUpper(family), err.Error()))
			continue
		}
		if family == FamilyIPv4 {
			info.IPv4 = true
		} else {
			info.IPv6 = true
		}
		info.Listeners = append(info.Listeners, listeners...)
	}

	info.Available = info.IPv4 || info.IPv6
	if !info.Available {
		info.Reason = "TCP listener inspection failed (" + strings.Join(failures, "; ") + ")."
	}
	sortListeners(info.Listeners)
	return info
}

func describeFamilyError(family string, err error) string {
	if tableErr, ok := err.(*TableError); ok {
		return tableErr.Error()
	}
	if err == nil {
		return "unknown error"
	}
	return err.Error()
}

// describeTableError maps documented Windows error codes to neutral text. It
// never includes a path or command line.
func describeTableError(family string, code uint32) string {
	label := strings.ToUpper(family)
	switch code {
	case 5:
		return "access denied while reading the " + label + " table"
	case 50:
		return "the " + label + " listener table is not supported on this system"
	case 87:
		return "Windows rejected the " + label + " table request as invalid"
	case 122:
		return "the " + label + " table kept changing size"
	default:
		return fmt.Sprintf("Windows error %d while reading the %s table", code, label)
	}
}

func sortListeners(listeners []model.TCPListener) {
	sort.SliceStable(listeners, func(i, j int) bool {
		a, b := listeners[i], listeners[j]
		if a.Port != b.Port {
			return a.Port < b.Port
		}
		if a.Family != b.Family {
			return a.Family < b.Family
		}
		if a.Address != b.Address {
			return a.Address < b.Address
		}
		if a.PID != b.PID {
			return a.PID < b.PID
		}
		return a.Protocol < b.Protocol
	})
}
