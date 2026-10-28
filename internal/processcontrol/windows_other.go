//go:build !windows

package processcontrol

import (
	"fmt"
	"syscall"
	"time"
)

// Stub implementations for non-Windows platforms

func openProcess(pid int32, access uint32) (syscall.Handle, error) {
	return 0, fmt.Errorf("process control not supported on this platform")
}

func closeHandle(handle syscall.Handle) error {
	return fmt.Errorf("process control not supported on this platform")
}

func getProcessTimes(handle syscall.Handle) (time.Time, error) {
	return time.Time{}, fmt.Errorf("process control not supported on this platform")
}

func getProcessExePath(handle syscall.Handle) (string, error) {
	return "", fmt.Errorf("process control not supported on this platform")
}

func isProcessCritical(handle syscall.Handle) (bool, error) {
	return false, fmt.Errorf("process control not supported on this platform")
}

func terminateProcess(handle syscall.Handle, exitCode uint32) error {
	return fmt.Errorf("process control not supported on this platform")
}

func getPriorityClass(handle syscall.Handle) (PriorityClass, error) {
	return 0, fmt.Errorf("process control not supported on this platform")
}

func setPriorityClass(handle syscall.Handle, priority PriorityClass) error {
	return fmt.Errorf("process control not supported on this platform")
}

func sendGracefulClose(pid int32) (int, error) {
	return 0, fmt.Errorf("process control not supported on this platform")
}

func isProcessRunning(handle syscall.Handle) (bool, error) {
	return false, fmt.Errorf("process control not supported on this platform")
}

func waitForProcessExit(handle syscall.Handle, timeoutMs uint32) (bool, error) {
	return false, fmt.Errorf("process control not supported on this platform")
}
