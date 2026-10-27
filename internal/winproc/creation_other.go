//go:build !windows

package winproc

import "time"

// CreationTime is unavailable away from Windows. Returning false keeps every
// caller's "identity cannot be verified" path explicit.
func CreationTime(pid int32) (time.Time, bool) {
	_ = pid
	return time.Time{}, false
}
