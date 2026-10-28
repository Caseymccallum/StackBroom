package processcontrol

import "time"

// NativeActions is the deterministic seam for process lifecycle tests.
// Production uses DefaultNativeActions; tests provide fakes.
type NativeActions struct {
	OpenVerified      func(pid int32, access uint32, expected time.Time) (Handle, ProcessIdentity, error)
	Close             func(Handle) error
	Terminate         func(Handle, uint32) error
	Wait              func(Handle, time.Duration) (bool, error)
	SendGracefulClose func(pid int32, expected time.Time) (int, error)
	GetPriority       func(Handle) (PriorityClass, error)
	SetPriority       func(Handle, PriorityClass) error
}
