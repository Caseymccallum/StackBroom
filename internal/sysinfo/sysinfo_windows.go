//go:build windows

package sysinfo

import (
	"context"
	"fmt"
	"sort"
	"syscall"
	"time"
	"unsafe"

	"github.com/Caseymccallum/StackBroom/internal/winproc"
)

var (
	sysKernel32 = syscall.NewLazyDLL("kernel32.dll")
	sysNtdll    = syscall.NewLazyDLL("ntdll.dll")

	procGetSystemTimes        = sysKernel32.NewProc("GetSystemTimes")
	procGetLogicalDrives      = sysKernel32.NewProc("GetLogicalDrives")
	procGetDriveTypeW         = sysKernel32.NewProc("GetDriveTypeW")
	procGetDiskFreeSpaceExW   = sysKernel32.NewProc("GetDiskFreeSpaceExW")
	procGetVolumeInformationW = sysKernel32.NewProc("GetVolumeInformationW")
	procRtlGetVersion         = sysNtdll.NewProc("RtlGetVersion")
)

const (
	driveFixed      = 3
	cpuSamplePeriod = 300 * time.Millisecond
)

type osVersionInfoW struct {
	Size       uint32
	Major      uint32
	Minor      uint32
	Build      uint32
	PlatformID uint32
	CSDVersion [128]uint16
}

func collectSystem(ctx context.Context) Metrics {
	result := baseMetrics()

	if total, available, ok := winproc.HostMemory(); ok {
		result.TotalMemoryBytes = total
		result.AvailableMemoryBytes = available
		result.MemoryKnown = true
	} else {
		result.Warnings = append(result.Warnings, "Physical memory could not be read.")
	}

	if major, minor, build, ok := windowsVersion(); ok {
		result.WindowsVersion = windowsVersionLabel(major, minor, build)
		result.BuildNumber = build
	} else {
		result.Warnings = append(result.Warnings, "Windows version could not be read.")
	}

	if percent, ok := sampleCPU(ctx); ok {
		result.CPUPercent = percent
		result.CPUKnown = true
	} else if ctx.Err() == nil {
		result.Warnings = append(result.Warnings, "CPU usage could not be sampled.")
	}

	volumes, warnings := fixedVolumes()
	result.Volumes = volumes
	result.Warnings = append(result.Warnings, warnings...)
	return result
}

func windowsVersion() (major, minor, build uint32, ok bool) {
	var info osVersionInfoW
	info.Size = uint32(unsafe.Sizeof(info))
	r, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&info)))
	if r != 0 {
		return 0, 0, 0, false
	}
	return info.Major, info.Minor, info.Build, true
}

func readSystemTimes() (cpuTimes, bool) {
	var idle, kernel, user syscall.Filetime
	r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if r == 0 {
		return cpuTimes{}, false
	}
	return cpuTimes{
		Idle:   uint64(idle.HighDateTime)<<32 | uint64(idle.LowDateTime),
		Kernel: uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime),
		User:   uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime),
	}, true
}

func sampleCPU(ctx context.Context) (float64, bool) {
	first, ok := readSystemTimes()
	if !ok {
		return 0, false
	}
	timer := time.NewTimer(cpuSamplePeriod)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return 0, false
	case <-timer.C:
	}
	second, ok := readSystemTimes()
	if !ok {
		return 0, false
	}
	return cpuUsage(first, second)
}

func fixedVolumes() ([]Volume, []string) {
	mask, _, _ := procGetLogicalDrives.Call()
	roots := driveRoots(uint32(mask))
	if len(roots) == 0 {
		return []Volume{}, []string{"No local volumes could be enumerated."}
	}

	volumes := make([]Volume, 0, len(roots))
	var warnings []string
	for _, root := range roots {
		rootPtr, err := syscall.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		driveType, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(rootPtr)))
		if uint32(driveType) != driveFixed {
			continue
		}

		var freeAvailable, total, totalFree uint64
		r, _, _ := procGetDiskFreeSpaceExW.Call(
			uintptr(unsafe.Pointer(rootPtr)),
			uintptr(unsafe.Pointer(&freeAvailable)),
			uintptr(unsafe.Pointer(&total)),
			uintptr(unsafe.Pointer(&totalFree)),
		)
		if r == 0 || total == 0 {
			warnings = append(warnings, fmt.Sprintf("Volume %s could not be measured.", root))
			continue
		}

		label, fileSystem := volumeInformation(rootPtr)
		used, percent := volumeUsage(total, freeAvailable, totalFree)
		volumes = append(volumes, Volume{
			Root:        root,
			Label:       label,
			FileSystem:  fileSystem,
			TotalBytes:  total,
			FreeBytes:   freeAvailable,
			UsedBytes:   used,
			UsedPercent: percent,
		})
	}
	sort.Slice(volumes, func(i, j int) bool { return volumes[i].Root < volumes[j].Root })
	return volumes, warnings
}

func volumeInformation(root *uint16) (label, fileSystem string) {
	var labelBuf [261]uint16
	var fsBuf [261]uint16
	r, _, _ := procGetVolumeInformationW.Call(
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&labelBuf[0])),
		uintptr(len(labelBuf)),
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&fsBuf[0])),
		uintptr(len(fsBuf)),
	)
	if r == 0 {
		return "", ""
	}
	return syscall.UTF16ToString(labelBuf[:]), syscall.UTF16ToString(fsBuf[:])
}
