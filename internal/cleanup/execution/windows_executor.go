//go:build windows && cleanup_execution

package execution

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/Caseymccallum/StackBroom/internal/storage"
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	clsidFileOperation = guid{0x3AD05575, 0x8857, 0x4850, [8]byte{0x92, 0x77, 0x11, 0xB8, 0x5B, 0xDB, 0x8E, 0x09}}
	iidFileOperation   = guid{0x947AAB5F, 0x0A5C, 0x4C13, [8]byte{0xB4, 0xD6, 0x4B, 0xF7, 0x83, 0x6F, 0xC9, 0xF8}}
	iidShellItem       = guid{0x43826D1E, 0xE718, 0x42EE, [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}

	modOle32Execution    = syscall.NewLazyDLL("ole32.dll")
	modShell32Execution  = syscall.NewLazyDLL("shell32.dll")
	procCoInitializeEx   = modOle32Execution.NewProc("CoInitializeEx")
	procCoUninitialize   = modOle32Execution.NewProc("CoUninitialize")
	procCoCreateInstance = modOle32Execution.NewProc("CoCreateInstance")
	procSHCreateItem     = modShell32Execution.NewProc("SHCreateItemFromParsingName")
)

const (
	coinitApartmentThreaded = 0x2
	clsctxInprocServer      = 0x1
	sOK                     = 0
	sFalse                  = 1
	fofAllowUndo            = 0x00000040
	fofNoConfirmation       = 0x00000010
	fofNoErrorUI            = 0x00000400
	fofSilent               = 0x00000004
	fofxAddUndoRecord       = 0x20000000
	fofxRecycleOnDelete     = 0x00080000
	fofxEarlyFailure        = 0x00100000
)

type windowsExecutor struct{}

func NewNativeExecutor() Executor { return windowsExecutor{} }

func (windowsExecutor) Execute(ctx context.Context, execution *Context, targets []Target, progress func(Progress)) (Result, error) {
	if os.Getenv("STACKBROOM_ENABLE_CLEANUP_EXECUTION") != "1" {
		return Result{State: StateFailed}, ErrDisabled
	}
	if execution == nil {
		return Result{State: StateFailed}, fmt.Errorf("missing execution context")
	}
	// Defense in depth: direct native calls cannot bypass the engine's recovery
	// gate. No COM activation occurs until a verified provider exists.
	if err := NewRecoveryVerifier().Available(); err != nil {
		return Result{State: StateFailed, ReasonCode: "recovery_unavailable"}, err
	}
	return executeWindowsSTA(ctx, targets, progress)
}

// Called only after the native gates. Tests never invoke this mutation worker.
func executeWindowsSTA(ctx context.Context, targets []Target, progress func(Progress)) (Result, error) {
	if ctx.Err() != nil {
		return Result{State: StateCancelled}, ctx.Err()
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if failedHRESULT(hr) {
		return Result{State: StateFailed, ReasonCode: "com_initialize", Message: hresult(hr)}, fmt.Errorf("COM initialization failed")
	}
	defer procCoUninitialize.Call()
	var fileOperation *comInterface
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOperation)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidFileOperation)), uintptr(unsafe.Pointer(&fileOperation)),
	)
	if failedHRESULT(hr) || fileOperation == nil {
		return Result{State: StateFailed, ReasonCode: "com_create", Message: hresult(hr)}, fmt.Errorf("IFileOperation creation failed")
	}
	var sinks []*deleteSink
	defer func() {
		releaseCOM(fileOperation)
		for _, sink := range sinks {
			sink.close()
		}
	}()
	flags := uintptr(fofAllowUndo | fofNoConfirmation | fofNoErrorUI | fofSilent | fofxAddUndoRecord | fofxRecycleOnDelete | fofxEarlyFailure)
	if hr = comCall(fileOperation, slotSetOperationFlags, flags); failedHRESULT(hr) {
		return Result{State: StateFailed, ReasonCode: "com_flags", Message: hresult(hr)}, fmt.Errorf("IFileOperation flags failed")
	}
	items := make([]ItemResult, 0, len(targets))
	for _, target := range targets {
		if ctx.Err() != nil {
			return Result{State: StateCancelled, Items: items, Aborted: true, ReasonCode: "cancelled"}, ctx.Err()
		}
		closeGuard, guardErr := storage.GuardCleanupAncestors(target.File)
		if guardErr != nil {
			return Result{State: StateFailed, Items: items, ReasonCode: "ancestor_guard"}, guardErr
		}
		defer closeGuard()
		pathPointer, err := syscall.UTF16PtrFromString(target.File.Path)
		if err != nil {
			items = append(items, ItemResult{TargetID: target.ID, Status: ItemFailed, ReasonCode: "path", Message: "target path is not usable"})
			return Result{State: StateFailed, Items: items, ReasonCode: "path"}, err
		}
		var shellItem *comInterface
		hr, _, _ = procSHCreateItem.Call(uintptr(unsafe.Pointer(pathPointer)), 0, uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&shellItem)))
		if failedHRESULT(hr) || shellItem == nil {
			items = append(items, ItemResult{TargetID: target.ID, Status: ItemFailed, ReasonCode: "shell_item", Message: hresult(hr)})
			return Result{State: StateFailed, Items: items, ReasonCode: "shell_item"}, fmt.Errorf("shell item creation failed")
		}
		sink := newDeleteSink(ctx, target, 0, len(targets), progress)
		sinks = append(sinks, sink)
		hr = comCall(fileOperation, slotDeleteItem, uintptr(unsafe.Pointer(shellItem)), sink.pointer())
		releaseCOM(shellItem)
		if failedHRESULT(hr) {
			items = append(items, ItemResult{TargetID: target.ID, Status: ItemFailed, ReasonCode: "queue", Message: hresult(hr), HRESULT: uint32(hr)})
			return Result{State: StateFailed, Items: items, ReasonCode: "queue"}, fmt.Errorf("target could not be queued")
		}
		items = append(items, ItemResult{TargetID: target.ID, Status: ItemQueued, LogicalBytes: target.File.LogicalBytes, AllocatedBytes: target.File.AllocatedBytes, AllocatedKnown: target.File.AllocatedKnown})
	}
	if ctx.Err() != nil {
		return Result{State: StateCancelled, Items: items, Aborted: true, ReasonCode: "cancelled"}, ctx.Err()
	}
	var aborted int32
	completed := 0
	// All callbacks share the actual completion count, independent of queue order.
	for _, sink := range sinks {
		sink.progress = func(p Progress) {
			completed++
			p.Completed = completed
			if progress != nil {
				progress(p)
			}
		}
	}
	performed, queried := performAndQuery(func(slot uintptr, args ...uintptr) uintptr { return comCall(fileOperation, slot, args...) }, uintptr(unsafe.Pointer(&aborted)))
	for i, sink := range sinks {
		if sink.completed {
			items[i] = sink.item
		}
	}
	if uint32(queried) != sOK {
		return Result{State: StateFailed, Items: items, Aborted: true, ReasonCode: "abort_query", Message: hresult(queried)}, fmt.Errorf("abort status could not be read")
	}
	if failedHRESULT(performed) {
		return Result{State: StateFailed, Items: items, Aborted: aborted != 0, ReasonCode: "perform", Message: hresult(performed)}, fmt.Errorf("IFileOperation perform failed")
	}
	return Result{State: StateVerifying, Items: items, Aborted: aborted != 0}, nil
}

type comInterface struct{ vtable *[23]uintptr }

func comCall(object *comInterface, index uintptr, args ...uintptr) uintptr {
	method := object.vtable[index]
	callArgs := append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)
	result, _, _ := syscall.SyscallN(method, callArgs...)
	runtime.KeepAlive(object)
	return result
}

func releaseCOM(object *comInterface) {
	if object != nil {
		comCall(object, slotRelease)
	}
}

func hresult(value uintptr) string {
	return fmt.Sprintf("HRESULT 0x%08x", uint32(value))
}

type unavailableVerifier struct{}

func (unavailableVerifier) Available() error {
	return fmt.Errorf("Recycle Bin verification sink is not enabled")
}

func (unavailableVerifier) Verify(context.Context, Target) (RecoveryEvidence, error) {
	return RecoveryEvidence{}, fmt.Errorf("Recycle Bin verification is unavailable")
}

func NewRecoveryVerifier() RecoveryVerifier { return unavailableVerifier{} }
