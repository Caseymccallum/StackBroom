//go:build windows && cleanup_execution

package execution

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/Caseymccallum/StackBroom/internal/storage"
)

var iidProgressSink = guid{0x04b0f1a7, 0x9490, 0x44bc, [8]byte{0x96, 0xe1, 0x42, 0x96, 0xa3, 0x12, 0x52, 0xe2}}
var iidUnknown = guid{0, 0, 0, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}

const hrAbort uintptr = 0x80004004
const hrNoInterface uintptr = 0x80004002
const hrPointer uintptr = 0x80004003

// Only ABI scalars cross the COM boundary. Context and Go callbacks live in a
// registry, not in native-readable memory. The owner pins this object until
// IFileOperation releases its references, on the same STA.
type sinkABI struct {
	vtable uintptr
	refs   int32
}
type deleteSink struct {
	abi                    sinkABI
	pin                    runtime.Pinner
	ctx                    context.Context
	target                 Target
	item                   ItemResult
	completed              bool
	recycleItem            *comInterface // retained only in this apartment; never a JSON token
	progress               func(Progress)
	completedBefore, total int
}

var sinkRegistry sync.Map
var sinkVtable = [...]uintptr{
	syscall.NewCallback(sinkQuery), syscall.NewCallback(sinkAddRef), syscall.NewCallback(sinkRelease),
	syscall.NewCallback(sinkStart), syscall.NewCallback(sinkFinish),
	syscall.NewCallback(sinkPreRename), syscall.NewCallback(sinkPostRename),
	syscall.NewCallback(sinkPreMoveCopy), syscall.NewCallback(sinkPostMoveCopy),
	syscall.NewCallback(sinkPreMoveCopy), syscall.NewCallback(sinkPostMoveCopy),
	syscall.NewCallback(sinkPreDelete), syscall.NewCallback(sinkPostDelete),
	syscall.NewCallback(sinkPreNew), syscall.NewCallback(sinkPostNew),
	syscall.NewCallback(sinkUpdate), syscall.NewCallback(sinkTimer),
	syscall.NewCallback(sinkTimer), syscall.NewCallback(sinkTimer),
}

func newDeleteSink(ctx context.Context, target Target, before, total int, progress func(Progress)) *deleteSink {
	s := &deleteSink{ctx: ctx, target: target, completedBefore: before, total: total, progress: progress, item: ItemResult{TargetID: target.ID, Status: ItemQueued}}
	s.abi.vtable = uintptr(unsafe.Pointer(&sinkVtable[0]))
	s.abi.refs = 1
	s.pin.Pin(&s.abi)
	sinkRegistry.Store(s.pointer(), s)
	return s
}
func (s *deleteSink) pointer() uintptr { return uintptr(unsafe.Pointer(&s.abi)) }
func (s *deleteSink) close() {
	sinkRelease(s.pointer()) // Drop the owner's reference; COM may still own one.
}
func lookupSink(this uintptr) *deleteSink {
	v, ok := sinkRegistry.Load(this)
	if !ok {
		return nil
	}
	return v.(*deleteSink)
}
func sinkQuery(this uintptr, iid *guid, out *uintptr) uintptr {
	if out == nil {
		return hrPointer
	}
	*out = 0
	if iid == nil {
		return hrPointer
	}
	requested := *iid
	if requested != iidUnknown && requested != iidProgressSink {
		return hrNoInterface
	}
	*out = this
	sinkAddRef(this)
	return 0
}
func sinkAddRef(this uintptr) uintptr {
	if s := lookupSink(this); s != nil {
		return uintptr(atomic.AddInt32(&s.abi.refs, 1))
	}
	return 0
}
func sinkRelease(this uintptr) uintptr {
	if s := lookupSink(this); s != nil {
		refs := atomic.AddInt32(&s.abi.refs, -1)
		if refs == 0 {
			if s.recycleItem != nil {
				releaseCOM(s.recycleItem)
				s.recycleItem = nil
			}
			sinkRegistry.Delete(this)
			s.pin.Unpin()
		}
		return uintptr(refs)
	}
	return 0
}
func sinkCancel(this uintptr) uintptr {
	s := lookupSink(this)
	if s == nil || s.ctx.Err() != nil {
		return hrAbort
	}
	return 0
}
func sinkStart(this uintptr) uintptr                                              { return sinkCancel(this) }
func sinkFinish(this, hr uintptr) uintptr                                         { return 0 }
func sinkPreRename(this, flags, item, name uintptr) uintptr                       { return hrAbort }
func sinkPostRename(this, flags, item, name, hr, newItem uintptr) uintptr         { return hrAbort }
func sinkPreMoveCopy(this, flags, item, dest, name uintptr) uintptr               { return hrAbort }
func sinkPostMoveCopy(this, flags, item, dest, name, hr, newItem uintptr) uintptr { return hrAbort }
func sinkPreNew(this, flags, dest, name uintptr) uintptr                          { return hrAbort }
func sinkPostNew(this, flags, dest, name, template, attributes, hr, newItem uintptr) uintptr {
	return hrAbort
}
func sinkTimer(this uintptr) uintptr                { return sinkCancel(this) }
func sinkUpdate(this, total, soFar uintptr) uintptr { return sinkCancel(this) }
func sinkPreDelete(this, flags, item uintptr) uintptr {
	if sinkCancel(this) != 0 {
		return hrAbort
	}
	s := lookupSink(this)
	// TSF_DELETE_RECYCLE_IF_POSSIBLE is necessary but is not a no-fallback proof.
	if flags&0x80 == 0 || storage.ValidateCleanupTarget(&s.target.File) != nil {
		return hrAbort
	}
	return 0
}
func sinkPostDelete(this, flags, item, hr uintptr, newItem *comInterface) uintptr {
	s := lookupSink(this)
	if s == nil {
		return hrAbort
	}
	if s.completed {
		return hrAbort
	}
	s.completed = true
	s.item.HRESULT = uint32(hr)
	s.item.Status = ItemUnknown
	s.item.ReasonCode = "delete_status_unknown"
	if failedHRESULT(hr) {
		s.item.Status = ItemFailed
		s.item.ReasonCode = "delete_failed"
		if uint32(hr) == 0x80070005 {
			s.item.Status = ItemDenied
		}
	} else if uint32(hr) == 0 {
		s.item.Status = ItemRemovedUnverified
		s.item.ReasonCode = "recovery_unverified"
		if newItem != nil {
			comCall(newItem, 1) // AddRef: retain beyond the callback, same STA only.
			s.recycleItem = newItem
			// This pointer is evidence of a Shell recycle result, not yet a durable,
			// identity-bound recovery token. Never claim ItemRecycled here.
			s.item.ReasonCode = "recycle_item_observed"
		}
	}
	if s.progress != nil {
		s.progress(Progress{State: StateExecuting, TargetID: s.target.ID, Completed: s.completedBefore + 1, Total: s.total})
	}
	return sinkCancel(this)
}
