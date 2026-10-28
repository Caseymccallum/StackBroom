//go:build !windows

package processcontrol

import (
	"fmt"
	"time"
)

// Stub implementations for non-Windows platforms

func openProcess(pid int32, access uint32) (Handle, error) {
	return 0, fmt.Errorf("process control not supported on this platform")
}

func closeHandle(handle Handle) error {
	return fmt.Errorf("process control not supported on this platform")
}

func getProcessTimes(handle Handle) (time.Time, error) {
	return time.Time{}, fmt.Errorf("process control not supported on this platform")
}

func getProcessExePath(handle Handle) (string, error) {
	return "", fmt.Errorf("process control not supported on this platform")
}

func isProcessCritical(handle Handle) (bool, error) {
	return false, fmt.Errorf("process control not supported on this platform")
}

func terminateProcess(handle Handle, exitCode uint32) error {
	return fmt.Errorf("process control not supported on this platform")
}

func getPriorityClass(handle Handle) (PriorityClass, error) {
	return 0, fmt.Errorf("process control not supported on this platform")
}

func setPriorityClass(handle Handle, priority PriorityClass) error {
	return fmt.Errorf("process control not supported on this platform")
}

func sendGracefulClose(pid int32, expected time.Time) (int, error) {
	return 0, fmt.Errorf("process control not supported on this platform")
}

func isProcessRunning(handle Handle) (bool, error) {
	return false, fmt.Errorf("process control not supported on this platform")
}

func waitForProcessExit(handle Handle, timeoutMs uint32) (bool, error) {
	return false, fmt.Errorf("process control not supported on this platform")
}
