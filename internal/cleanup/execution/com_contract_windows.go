//go:build windows && cleanup_execution

package execution

// Zero-based slots, including all three IUnknown entries. Audited against the
// installed Windows SDK ShObjIdl_core.h and Microsoft's win32metadata header.
const (
	slotRelease                 = 2
	slotSetOperationFlags       = 5
	slotDeleteItem              = 18
	slotPerformOperations       = 21
	slotGetAnyOperationsAborted = 22
)

func failedHRESULT(hr uintptr) bool { return int32(uint32(hr)) < 0 }

// Always query aborted state, including after a failed PerformOperations.
// An unreadable BOOL is conservatively treated as an abort.
func performAndQuery(call func(uintptr, ...uintptr) uintptr, abortedPointer uintptr) (uintptr, uintptr) {
	performed := call(slotPerformOperations)
	queried := call(slotGetAnyOperationsAborted, abortedPointer)
	return performed, queried
}
