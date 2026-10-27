// Package sysinfo collects read-only host metrics for the desktop shell.
//
// It deliberately contains no GUI dependency and no mutation path. Windows
// syscalls live in sysinfo_windows.go; the pure calculations and the
// non-Windows fallback are platform-neutral.
package sysinfo

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"
)

// Volume is one fixed local volume. FreeBytes is the space Windows reports as
// available to the current user, not a claim about reclaimable space.
type Volume struct {
	Root        string
	Label       string
	FileSystem  string
	TotalBytes  uint64
	FreeBytes   uint64
	UsedBytes   uint64
	UsedPercent float64
}

// Metrics is one point-in-time host observation. Unknown values are flagged
// rather than zero-filled.
type Metrics struct {
	Hostname             string
	WindowsVersion       string
	BuildNumber          uint32
	LogicalProcessors    int
	TotalMemoryBytes     uint64
	AvailableMemoryBytes uint64
	MemoryKnown          bool
	CPUPercent           float64
	CPUKnown             bool
	Volumes              []Volume
	SampledAt            time.Time
	Warnings             []string
}

// Collect gathers host metrics. CPU sampling takes a short interval and honours
// context cancellation; no expensive probe is unbounded.
func Collect(ctx context.Context) Metrics {
	return collectSystem(ctx)
}

// cpuTimes holds the cumulative counters returned by GetSystemTimes.
type cpuTimes struct {
	Idle   uint64
	Kernel uint64
	User   uint64
}

// cpuUsage computes busy percentage from two cumulative samples. Windows counts
// idle time inside the kernel total, so busy = (kernel+user) - idle.
func cpuUsage(previous, current cpuTimes) (float64, bool) {
	kernel := current.Kernel - previous.Kernel
	user := current.User - previous.User
	idle := current.Idle - previous.Idle
	total := kernel + user
	if total == 0 || idle > total {
		return 0, false
	}
	busy := total - idle
	return float64(busy) / float64(total) * 100, true
}

// driveRoots decodes a GetLogicalDrives bitmask into Windows volume roots.
func driveRoots(mask uint32) []string {
	var roots []string
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		roots = append(roots, fmt.Sprintf("%c:\\", rune('A'+i)))
	}
	return roots
}

// volumeUsage reports used bytes and percentage. FreeBytes is the user's free
// space; the used figure uses total free, so reserved clusters are not counted
// as used. The two values are not guaranteed to sum exactly to total.
func volumeUsage(total, freeAvailable, totalFree uint64) (used uint64, percent float64) {
	if total == 0 {
		return 0, 0
	}
	if totalFree > total {
		totalFree = total
	}
	used = total - totalFree
	return used, float64(used) / float64(total) * 100
}

func windowsVersionLabel(major, minor, build uint32) string {
	switch {
	case major == 10 && build >= 22000:
		return fmt.Sprintf("Windows 11 (build %d)", build)
	case major == 10:
		return fmt.Sprintf("Windows 10 (build %d)", build)
	case major == 6 && minor == 3:
		return fmt.Sprintf("Windows 8.1 (build %d)", build)
	case major == 6 && minor == 2:
		return fmt.Sprintf("Windows 8 (build %d)", build)
	case major == 6 && minor == 1:
		return fmt.Sprintf("Windows 7 (build %d)", build)
	default:
		return fmt.Sprintf("Windows %d.%d (build %d)", major, minor, build)
	}
}

func baseMetrics() Metrics {
	name, _ := os.Hostname()
	return Metrics{
		Hostname:          name,
		LogicalProcessors: runtime.NumCPU(),
		Volumes:           []Volume{},
		SampledAt:         time.Now(),
		Warnings:          []string{},
	}
}
