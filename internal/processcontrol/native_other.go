//go:build !windows

package processcontrol

import (
	"fmt"
	"time"
)

// DefaultNativeActions reports unsupported operations outside Windows.
func DefaultNativeActions() NativeActions {
	unsupported := func() error {
		return fmt.Errorf("process control is only supported on Windows")
	}
	return NativeActions{
		OpenVerified: func(int32, uint32, time.Time) (Handle, ProcessIdentity, error) {
			return 0, ProcessIdentity{}, unsupported()
		},
		Close:             func(Handle) error { return unsupported() },
		Terminate:         func(Handle, uint32) error { return unsupported() },
		Wait:              func(Handle, time.Duration) (bool, error) { return false, unsupported() },
		SendGracefulClose: func(int32, time.Time) (int, error) { return 0, unsupported() },
		GetPriority:       func(Handle) (PriorityClass, error) { return 0, unsupported() },
		SetPriority:       func(Handle, PriorityClass) error { return unsupported() },
	}
}
