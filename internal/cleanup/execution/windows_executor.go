//go:build windows && cleanup_execution

package execution

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
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

func (windowsExecutor) Execute(ctx context.Context, _ *Context, targets []Target, progress func(Progress)) (Result, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if uint32(hr) != sOK && uint32(hr) != sFalse {
		return Result{State: StateFailed, ReasonCode: "com_initialize", Message: hresult(hr)}, fmt.Errorf("COM initialization failed")
	}
	defer procCoUninitialize.Call()
	var fileOperation uintptr
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOperation)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidFileOperation)), uintptr(unsafe.Pointer(&fileOperation)),
	)
	if uint32(hr) != sOK || fileOperation == 0 {
		return Result{State: StateFailed, ReasonCode: "com_create", Message: hresult(hr)}, fmt.Errorf("IFileOperation creation failed")
	}
	defer releaseCOM(fileOperation)
	flags := uintptr(fofAllowUndo | fofNoConfirmation | fofNoErrorUI | fofSilent | fofxAddUndoRecord | fofxRecycleOnDelete | fofxEarlyFailure)
	if hr = comCall(fileOperation, 5, flags); uint32(hr) != sOK {
		return Result{State: StateFailed, ReasonCode: "com_flags", Message: hresult(hr)}, fmt.Errorf("IFileOperation flags failed")
	}
	items := make([]ItemResult, 0, len(targets))
	for index, target := range targets {
		if ctx.Err() != nil {
			return Result{State: StateCancelled, Items: items, Aborted: true, ReasonCode: "cancelled"}, ctx.Err()
		}
		pathPointer, err := syscall.UTF16PtrFromString(target.File.Path)
		if err != nil {
			items = append(items, ItemResult{TargetID: target.ID, Status: ItemFailed, ReasonCode: "path", Message: "target path is not usable"})
			return Result{State: StateFailed, Items: items, ReasonCode: "path"}, err
		}
		var shellItem uintptr
		hr, _, _ = procSHCreateItem.Call(uintptr(unsafe.Pointer(pathPointer)), 0, uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&shellItem)))
		if uint32(hr) != sOK || shellItem == 0 {
			items = append(items, ItemResult{TargetID: target.ID, Status: ItemFailed, ReasonCode: "shell_item", Message: hresult(hr)})
			return Result{State: StateFailed, Items: items, ReasonCode: "shell_item"}, fmt.Errorf("shell item creation failed")
		}
		hr = comCall(fileOperation, 18, shellItem, 0)
		releaseCOM(shellItem)
		if uint32(hr) != sOK {
			items = append(items, ItemResult{TargetID: target.ID, Status: ItemFailed, ReasonCode: "queue", Message: hresult(hr), HRESULT: uint32(hr)})
			return Result{State: StateFailed, Items: items, ReasonCode: "queue"}, fmt.Errorf("target could not be queued")
		}
		items = append(items, ItemResult{TargetID: target.ID, Status: ItemQueued, LogicalBytes: target.File.LogicalBytes, AllocatedBytes: target.File.AllocatedBytes, AllocatedKnown: target.File.AllocatedKnown})
		if progress != nil {
			progress(Progress{State: StateExecuting, TargetID: target.ID, Completed: index + 1, Total: len(targets)})
		}
	}
	if ctx.Err() != nil {
		return Result{State: StateCancelled, Items: items, Aborted: true, ReasonCode: "cancelled"}, ctx.Err()
	}
	hr = comCall(fileOperation, 21)
	if uint32(hr) != sOK {
		return Result{State: StateFailed, Items: items, ReasonCode: "perform", Message: hresult(hr)}, fmt.Errorf("IFileOperation perform failed")
	}
	var aborted int32
	hr = comCall(fileOperation, 22, uintptr(unsafe.Pointer(&aborted)))
	if uint32(hr) != sOK {
		return Result{State: StateFailed, Items: items, ReasonCode: "abort_query", Message: hresult(hr)}, fmt.Errorf("abort status could not be read")
	}
	return Result{State: StateVerifying, Items: items, Aborted: aborted != 0}, nil
}

func comCall(object uintptr, index uintptr, args ...uintptr) uintptr {
	vtable := *(*uintptr)(unsafe.Pointer(object))
	method := *(*uintptr)(unsafe.Pointer(vtable + index*unsafe.Sizeof(uintptr(0))))
	callArgs := append([]uintptr{object}, args...)
	result, _, _ := syscall.SyscallN(method, callArgs...)
	return result
}

func releaseCOM(object uintptr) {
	if object != 0 {
		comCall(object, 2)
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
