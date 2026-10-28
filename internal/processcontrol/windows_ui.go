//go:build windows

package processcontrol

import (
	"fmt"
	"syscall"
	"unsafe"
)

const WM_CLOSE = 0x0010

// enumWindowsCallback is called for each window during enumeration
type enumWindowsCallback func(hwnd syscall.Handle, lParam uintptr) uintptr

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
func getWindowThreadProcessId(hwnd syscall.Handle) (int32, error) {
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
func postMessage(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) error {
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

// sendGracefulClose sends WM_CLOSE to all windows owned by the specified PID
func sendGracefulClose(pid int32) (int, error) {
	windowCount := 0
	var lastErr error

	callback := func(hwnd syscall.Handle, lParam uintptr) uintptr {
		ownerPID, err := getWindowThreadProcessId(hwnd)
		if err != nil {
			lastErr = err
			return 0
		}
		if ownerPID == pid {
			if err := postMessage(hwnd, WM_CLOSE, 0, 0); err != nil {
				lastErr = err
			} else {
				windowCount++
			}
		}
		return 1 // continue enumeration
	}

	if err := enumWindows(callback); err != nil {
		return 0, err
	}
	return windowCount, lastErr
}
