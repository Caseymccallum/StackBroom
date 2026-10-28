//go:build windows

package processcontrol

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modKernel32 = windows.NewLazySystemDLL("kernel32.dll")
	modUser32   = windows.NewLazySystemDLL("user32.dll")

	procOpenProcess               = modKernel32.NewProc("OpenProcess")
	procCloseHandle               = modKernel32.NewProc("CloseHandle")
	procGetProcessTimes           = modKernel32.NewProc("GetProcessTimes")
	procTerminateProcess          = modKernel32.NewProc("TerminateProcess")
	procGetPriorityClass          = modKernel32.NewProc("GetPriorityClass")
	procSetPriorityClass          = modKernel32.NewProc("SetPriorityClass")
	procQueryFullProcessImageName = modKernel32.NewProc("QueryFullProcessImageNameW")
	procIsProcessCritical         = modKernel32.NewProc("IsProcessCritical")
	procGetExitCodeProcess        = modKernel32.NewProc("GetExitCodeProcess")
	procWaitForSingleObject       = modKernel32.NewProc("WaitForSingleObject")
	procPostMessage               = modUser32.NewProc("PostMessageW")
	procEnumWindows               = modUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId  = modUser32.NewProc("GetWindowThreadProcessId")
)

// Process access rights
const (
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	PROCESS_TERMINATE                 = 0x0001
	PROCESS_SET_INFORMATION           = 0x0200
	PROCESS_QUERY_INFORMATION         = 0x0400
)

// FILETIME represents Windows FILETIME structure
type FILETIME struct {
	LowDateTime  uint32
	HighDateTime uint32
}

// filetimeToTime converts Windows FILETIME to Go time.Time
func filetimeToTime(ft FILETIME) time.Time {
	nsec := int64(ft.HighDateTime)<<32 + int64(ft.LowDateTime)
	unixNsec := (nsec - 116444736000000000) * 100
	return time.Unix(0, unixNsec)
}

// openProcess opens a process handle with specified access rights
func openProcess(pid int32, access uint32) (syscall.Handle, error) {
	handle, _, err := procOpenProcess.Call(uintptr(access), 0, uintptr(pid))
	if handle == 0 {
		return 0, fmt.Errorf("OpenProcess failed: %w", err)
	}
	return syscall.Handle(handle), nil
}

// closeHandle closes a process handle
func closeHandle(handle syscall.Handle) error {
	ret, _, err := procCloseHandle.Call(uintptr(handle))
	if ret == 0 {
		return fmt.Errorf("CloseHandle failed: %w", err)
	}
	return nil
}

// getProcessTimes retrieves process creation, exit, kernel and user times
func getProcessTimes(handle syscall.Handle) (createTime time.Time, err error) {
	var creation, exit, kernel, user FILETIME
	ret, _, err := procGetProcessTimes.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if ret == 0 {
		return time.Time{}, fmt.Errorf("GetProcessTimes failed: %w", err)
	}
	return filetimeToTime(creation), nil
}

// getProcessExePath retrieves the full executable path
func getProcessExePath(handle syscall.Handle) (string, error) {
	var buf [syscall.MAX_PATH]uint16
	size := uint32(len(buf))
	ret, _, err := procQueryFullProcessImageName.Call(
		uintptr(handle),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return "", fmt.Errorf("QueryFullProcessImageName failed: %w", err)
	}
	return syscall.UTF16ToString(buf[:size]), nil
}

// isProcessCritical checks if process is critical to system operation
func isProcessCritical(handle syscall.Handle) (bool, error) {
	var critical uint32
	ret, _, err := procIsProcessCritical.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&critical)),
	)
	if ret == 0 {
		return false, fmt.Errorf("IsProcessCritical failed: %w", err)
	}
	return critical != 0, nil
}

// terminateProcess forcefully terminates a process
func terminateProcess(handle syscall.Handle, exitCode uint32) error {
	ret, _, err := procTerminateProcess.Call(uintptr(handle), uintptr(exitCode))
	if ret == 0 {
		return fmt.Errorf("TerminateProcess failed: %w", err)
	}
	return nil
}

// getPriorityClass retrieves the priority class of a process
func getPriorityClass(handle syscall.Handle) (PriorityClass, error) {
	ret, _, err := procGetPriorityClass.Call(uintptr(handle))
	if ret == 0 {
		return 0, fmt.Errorf("GetPriorityClass failed: %w", err)
	}
	return PriorityClass(ret), nil
}

// setPriorityClass sets the priority class of a process
func setPriorityClass(handle syscall.Handle, priority PriorityClass) error {
	ret, _, err := procSetPriorityClass.Call(uintptr(handle), uintptr(priority))
	if ret == 0 {
		return fmt.Errorf("SetPriorityClass failed: %w", err)
	}
	return nil
}

// isProcessRunning checks if a process is still running
func isProcessRunning(handle syscall.Handle) (bool, error) {
	var exitCode uint32
	ret, _, err := procGetExitCodeProcess.Call(uintptr(handle), uintptr(unsafe.Pointer(&exitCode)))
	if ret == 0 {
		return false, fmt.Errorf("GetExitCodeProcess failed: %w", err)
	}
	// STILL_ACTIVE = 259
	return exitCode == 259, nil
}

// waitForProcessExit waits for a process to exit with a timeout
func waitForProcessExit(handle syscall.Handle, timeoutMs uint32) (bool, error) {
	ret, _, err := procWaitForSingleObject.Call(uintptr(handle), uintptr(timeoutMs))
	if ret == 0xFFFFFFFF { // WAIT_FAILED
		return false, fmt.Errorf("WaitForSingleObject failed: %w", err)
	}
	// WAIT_OBJECT_0 = 0 (process exited), WAIT_TIMEOUT = 258
	return ret == 0, nil
}
