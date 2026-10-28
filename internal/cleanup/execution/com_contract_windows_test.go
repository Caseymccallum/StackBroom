//go:build windows && cleanup_execution

package execution

import (
	"context"
	"os"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestInstalledSDKContract(t *testing.T) {
	header := os.Getenv("STACKBROOM_TEST_SDK_HEADER")
	if header == "" {
		t.Skip("set STACKBROOM_TEST_SDK_HEADER for independent SDK contract check")
	}
	raw, err := os.ReadFile(header)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "typedef struct IFileOperationVtbl")
	if start < 0 {
		t.Fatal("missing SDK vtable")
	}
	tail := text[start:]
	end := strings.Index(tail, "} IFileOperationVtbl")
	if end < 0 {
		t.Fatal("missing SDK vtable end")
	}
	tail = tail[:end]
	names := []string{"QueryInterface", "AddRef", "Release", "Advise", "Unadvise", "SetOperationFlags", "SetProgressMessage", "SetProgressDialog", "SetProperties", "SetOwnerWindow", "ApplyPropertiesToItem", "ApplyPropertiesToItems", "RenameItem", "RenameItems", "MoveItem", "MoveItems", "CopyItem", "CopyItems", "DeleteItem", "DeleteItems", "NewItem", "PerformOperations", "GetAnyOperationsAborted"}
	position := -1
	for _, name := range names {
		next := strings.Index(tail, "*"+name+" )")
		if next < 0 {
			next = strings.Index(tail, "*"+name+")")
		}
		if next <= position {
			t.Fatalf("SDK method order mismatch: %s", name)
		}
		position = next
	}
	if names[slotDeleteItem] != "DeleteItem" || names[slotPerformOperations] != "PerformOperations" || names[slotGetAnyOperationsAborted] != "GetAnyOperationsAborted" {
		t.Fatal("slot mismatch")
	}
	if !strings.Contains(strings.ToLower(text), "947aab5f-0a5c-4c13-b4d6-4bf7836fc9f8") {
		t.Fatal("SDK IID absent")
	}
	expected := guid{0x947aab5f, 0x0a5c, 0x4c13, [8]byte{0xb4, 0xd6, 0x4b, 0xf7, 0x83, 0x6f, 0xc9, 0xf8}}
	if iidFileOperation != expected || unsafe.Sizeof(guid{}) != 16 {
		t.Fatal("GUID mismatch")
	}
	if !strings.Contains(strings.ToLower(text), "04b0f1a7-9490-44bc-96e1-4296a31252e2") || !strings.Contains(strings.ToLower(text), "43826d1e-e718-42ee-bc55-a1e261c37bfe") {
		t.Fatal("SDK sink/ShellItem IID absent")
	}
	if iidProgressSink != (guid{0x04b0f1a7, 0x9490, 0x44bc, [8]byte{0x96, 0xe1, 0x42, 0x96, 0xa3, 0x12, 0x52, 0xe2}}) || iidShellItem != (guid{0x43826d1e, 0xe718, 0x42ee, [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}) {
		t.Fatal("sink/ShellItem GUID mismatch")
	}
}
func TestHRESULTAndAlwaysQueryAbort(t *testing.T) {
	for _, hr := range []uintptr{0, 1, 0x270005} {
		if failedHRESULT(hr) {
			t.Fatalf("success %#x", hr)
		}
	}
	if !failedHRESULT(0x80004005) {
		t.Fatal("failure accepted")
	}
	var slots []uintptr
	perform, query := performAndQuery(func(slot uintptr, args ...uintptr) uintptr { slots = append(slots, slot); return 0x80004005 }, 0)
	if perform != 0x80004005 || query != 0x80004005 || len(slots) != 2 || slots[1] != slotGetAnyOperationsAborted {
		t.Fatal("abort query skipped")
	}
}
func TestSinkCallbacksWithoutShellOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var completed int
	sink := newDeleteSink(ctx, Target{ID: "test"}, 0, 3, func(p Progress) { completed = p.Completed })
	defer sink.close()
	// Dispatch through actual ABI callbacks, without activating COM or Shell.
	hr, _, _ := syscall.SyscallN(sinkVtable[11], sink.pointer(), 0, 0)
	if hr != hrAbort {
		t.Fatal("non-recycle request accepted")
	}
	var out uintptr
	hr, _, _ = syscall.SyscallN(sinkVtable[0], sink.pointer(), uintptr(unsafe.Pointer(&iidProgressSink)), uintptr(unsafe.Pointer(&out)))
	if hr != 0 || out != sink.pointer() {
		t.Fatal("QueryInterface")
	}
	sinkRelease(sink.pointer())
	hr, _, _ = syscall.SyscallN(sinkVtable[12], sink.pointer(), 0, 0, 0, 0)
	if hr != 0 || completed != 1 || sink.item.Status != ItemRemovedUnverified {
		t.Fatalf("callback = %#x %+v %d", hr, sink.item, completed)
	}
	if sinkPostDelete(sink.pointer(), 0, 0, 0, nil) != hrAbort {
		t.Fatal("duplicate callback counted")
	}
	cancel()
	if sinkUpdate(sink.pointer(), 100, 2) != hrAbort {
		t.Fatal("cancellation ignored")
	}
}
func TestNativeExecutorRefusesBeforeCOM(t *testing.T) {
	t.Setenv("STACKBROOM_ENABLE_CLEANUP_EXECUTION", "1")
	result, err := NewNativeExecutor().Execute(context.Background(), &Context{}, nil, nil)
	if err == nil || result.ReasonCode != "recovery_unavailable" {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestSinkRetainsSyntheticRecycleItemWithoutClaimingRecovery(t *testing.T) {
	refs := 1
	vtable := new([23]uintptr)
	vtable[1] = syscall.NewCallback(func(this uintptr) uintptr { refs++; return uintptr(refs) })
	vtable[2] = syscall.NewCallback(func(this uintptr) uintptr { refs--; return uintptr(refs) })
	item := &comInterface{vtable: vtable}
	sink := newDeleteSink(context.Background(), Target{ID: "test"}, 0, 1, nil)
	sinkPostDelete(sink.pointer(), 0, 0, 0, item)
	if refs != 2 || sink.item.Status == ItemRecycled || sink.recycleItem != item {
		t.Fatal("invalid recycle evidence lifetime or success claim")
	}
	sink.close()
	if refs != 1 {
		t.Fatal("recycle reference leaked")
	}
}
