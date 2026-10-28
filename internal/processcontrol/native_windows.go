//go:build windows

package processcontrol

import "time"

// DefaultNativeActions wires the lifecycle to the Windows implementation.
func DefaultNativeActions() NativeActions {
	return NativeActions{
		OpenVerified:      openVerifiedProcess,
		Close:             closeHandle,
		Terminate:         terminateProcess,
		Wait:              waitForProcessExitDuration,
		SendGracefulClose: sendGracefulClose,
		GetPriority:       getPriorityClass,
		SetPriority:       setPriorityClass,
	}
}

func waitForProcessExitDuration(handle Handle, timeout time.Duration) (bool, error) {
	return waitForProcessExit(handle, uint32(timeout.Milliseconds()))
}
