//go:build windows

package processcontrol

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

const WM_CLOSE = 0x0010

// enumWindowsCallback is called for each window during enumeration
type enumWindowsCallback func(hwnd Handle, lParam uintptr) uintptr

// enumWindows enumerates all top-level windows
func enumWindows(callback enumWindowsCallback) error {
	ret, _, err := procEnumWindows.Call(
		syscall.NewCallback(callback),
		0,
	)
	if ret == 0 {
		return fmt.Errorf("EnumWindows failed: %w", err)
	}
	return nil
}

// getWindowThreadProcessId retrieves the PID that owns a window
func getWindowThreadProcessId(hwnd Handle) (int32, error) {
	var pid int32
	ret, _, err := procGetWindowThreadProcessId.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&pid)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("GetWindowThreadProcessId failed: %w", err)
	}
	return pid, nil
}

// postMessage sends a message to a window (used for WM_CLOSE)
func postMessage(hwnd Handle, msg uint32, wParam, lParam uintptr) error {
	ret, _, err := procPostMessage.Call(
		uintptr(hwnd),
		uintptr(msg),
		wParam,
		lParam,
	)
	if ret == 0 {
		return fmt.Errorf("PostMessage failed: %w", err)
	}
	return nil
}

// sendGracefulClose collects windows owned by the PID, re-verifies the process
// identity, then delivers WM_CLOSE. It never posts to a window belonging to a
// process that reused the PID.
func sendGracefulClose(pid int32, expected time.Time) (int, error) {
	var windows []Handle

	callback := func(hwnd Handle, lParam uintptr) uintptr {
		ownerPID, err := getWindowThreadProcessId(hwnd)
		if err != nil {
			return 1
		}
		if ownerPID == pid {
			windows = append(windows, hwnd)
		}
		return 1 // continue enumeration
	}

	if err := enumWindows(callback); err != nil {
		return 0, err
	}
	if len(windows) == 0 {
		return 0, nil
	}

	handle, err := openProcess(pid, PROCESS_QUERY_LIMITED_INFORMATION)
	if err != nil {
		return 0, fmt.Errorf("cannot verify window owner: %w", err)
	}
	created, err := getProcessTimes(handle)
	_ = closeHandle(handle)
	if err != nil {
		return 0, fmt.Errorf("cannot verify window owner: %w", err)
	}
	if !created.Equal(expected) {
		return 0, fmt.Errorf("window owner identity changed (PID reuse)")
	}

	windowCount := 0
	var lastErr error
	for _, hwnd := range windows {
		if err := postMessage(hwnd, WM_CLOSE, 0, 0); err != nil {
			lastErr = err
		} else {
			windowCount++
		}
	}
	return windowCount, lastErr
}
