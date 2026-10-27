//go:build windows

package winproc

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")

	procCreateToolhelp32Snapshot   = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW            = kernel32.NewProc("Process32FirstW")
	procProcess32NextW             = kernel32.NewProc("Process32NextW")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procGetProcessMemoryInfo       = psapi.NewProc("GetProcessMemoryInfo")
)

const (
	th32csSnapProcess       = 0x00000002
	processQueryLimitedInfo = 0x1000
	processQueryInformation = 0x0400
	processVMRead           = 0x0010
	imageNameBufferLength   = 1024
)

// processMemoryCountersEx mirrors PROCESS_MEMORY_COUNTERS_EX.
type processMemoryCountersEx struct {
	Size                    uint32
	PageFaultCount          uint32
	PeakWorkingSetSize      uintptr
	WorkingSetSize          uintptr
	QuotaPeakPagedPoolUsage uintptr
	QuotaPagedPoolUsage     uintptr
	QuotaPeakNonPagedPool   uintptr
	QuotaNonPagedPoolUsage  uintptr
	PagefileUsage           uintptr
	PeakPagefileUsage       uintptr
	PrivateUsage            uintptr
}

func collectProcesses(ctx context.Context, opts Options) (Result, error) {
	entries, err := snapshotProcesses()
	if err != nil {
		return Result{}, fmt.Errorf("enumerating Windows processes: %w", err)
	}

	res := Result{Source: SourceNone}
	procs := make([]model.Process, 0, len(entries))
	for _, e := range entries {
		p := model.Process{PID: e.PID, ParentPID: e.ParentPID, Name: e.Name}
		if path, ok := imagePath(e.PID); ok {
			p.ExePath = path
		}
		if ws, ok := workingSetBytes(e.PID); ok {
			p.WorkingSetBytes = ws
			p.MemoryKnown = true
		}
		procs = append(procs, p)
	}

	names := make(map[int32]string, len(procs))
	for _, p := range procs {
		names[p.PID] = p.Name
	}
	for i := range procs {
		if name, ok := names[procs[i].ParentPID]; ok {
			procs[i].ParentName = name
		}
	}

	if opts.EnrichCommandLines {
		enrich, warnings, err := queryCommandLines(ctx, opts)
		res.Warnings = append(res.Warnings, warnings...)
		switch {
		case err != nil:
			res.Source = SourceFailed
			res.SourceError = err.Error()
			res.Warnings = append(res.Warnings, "Command lines could not be read ("+err.Error()+"). Runtimes are listed, but roles and project associations are limited.")
		default:
			res.Source = SourceCIM
			ApplyEnrichment(procs, enrich)
		}
	} else {
		res.Source = SourceDisabled
		res.Warnings = append(res.Warnings, "Command-line inspection is disabled; roles, ports and project associations are limited.")
	}

	for i := range procs {
		Classify(&procs[i])
	}
	res.Processes = procs
	return res, nil
}

// snapshotProcesses walks the Toolhelp32 process list. The API is read-only and
// requires no elevation for process metadata.
func snapshotProcesses() ([]Entry, error) {
	// Misuse of the uintptr return is safe here: the handle is only used as an
	// opaque value and no Go pointer is retained by the kernel.
	snap, _, callErr := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == uintptr(syscall.InvalidHandle) {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed: %v", callErr)
	}
	defer closeHandle(syscall.Handle(snap))

	buf := make([]byte, processEntry32Size)
	*(*uint32)(unsafe.Pointer(&buf[0])) = processEntry32Size

	ok, _, callErr := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&buf[0])))
	if ok == 0 {
		if errors.Is(callErr, syscall.ERROR_NO_MORE_FILES) {
			return nil, nil
		}
		return nil, fmt.Errorf("Process32FirstW failed: %v", callErr)
	}

	var entries []Entry
	for {
		if e, valid := decodeProcessEntry32(buf); valid {
			entries = append(entries, e)
		}
		*(*uint32)(unsafe.Pointer(&buf[0])) = processEntry32Size
		ok, _, callErr = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&buf[0])))
		if ok == 0 {
			if callErr != nil && !errors.Is(callErr, syscall.ERROR_NO_MORE_FILES) {
				// A partial list is still useful; stop without failing the scan.
				break
			}
			break
		}
	}
	return entries, nil
}

func imagePath(pid int32) (string, bool) {
	h, err := openProcess(processQueryLimitedInfo, pid)
	if err != nil {
		return "", false
	}
	defer closeHandle(h)

	buf := make([]uint16, imageNameBufferLength)
	size := uint32(len(buf))
	r, _, _ := procQueryFullProcessImageNameW.Call(
		uintptr(h), 0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r == 0 || size == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf[:size]), true
}

func workingSetBytes(pid int32) (uint64, bool) {
	// Limited-information handles are tried first because they work for other
	// users' processes. The wider access mask is a fallback for older systems.
	for _, access := range []uint32{processQueryLimitedInfo, processQueryInformation | processVMRead} {
		h, err := openProcess(access, pid)
		if err != nil {
			continue
		}
		var counters processMemoryCountersEx
		counters.Size = uint32(unsafe.Sizeof(counters))
		r, _, _ := procGetProcessMemoryInfo.Call(
			uintptr(h),
			uintptr(unsafe.Pointer(&counters)),
			uintptr(counters.Size),
		)
		closeHandle(h)
		if r != 0 {
			return uint64(counters.WorkingSetSize), true
		}
	}
	return 0, false
}

func openProcess(access uint32, pid int32) (syscall.Handle, error) {
	h, _, err := procOpenProcess.Call(uintptr(access), 0, uintptr(pid))
	if h == 0 {
		return 0, err
	}
	return syscall.Handle(h), nil
}

func closeHandle(h syscall.Handle) {
	if h != 0 && h != syscall.InvalidHandle {
		procCloseHandle.Call(uintptr(h))
	}
}
