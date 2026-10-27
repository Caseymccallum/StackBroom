//go:build !windows

package winproc

// HostMemory is unavailable outside Windows; callers must show "Unknown"
// rather than a zero total.
func HostMemory() (total uint64, available uint64, ok bool) {
	return 0, 0, false
}
