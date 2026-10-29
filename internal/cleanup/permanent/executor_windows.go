//go:build windows && cleanup_execution

package permanent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
	"github.com/Caseymccallum/StackBroom/internal/storage"
)

const (
	deleteAccess             = 0x00010000
	fileReadAttributes       = 0x00000080
	openExisting             = 3
	fileFlagOpenReparsePoint = 0x00200000
	fileDispositionInfo      = 4
	fileAttributeReadOnly    = 0x00000001
	fileAttributeHidden      = 0x00000002
	fileAttributeSystem      = 0x00000004
	fileAttributeDirectory   = 0x00000010
	fileAttributeSparse      = 0x00000200
	fileAttributeReparse     = 0x00000400
	fileAttributeOffline     = 0x00001000
	fileAttributeRecallOpen  = 0x00040000
	fileAttributeRecallData  = 0x00400000
	errorAccessDenied        = 5
	errorSharingViolation    = 32
	errorCannotDelete        = 600
)

var (
	kernel32Permanent = syscall.NewLazyDLL("kernel32.dll")
	advapi32Permanent = syscall.NewLazyDLL("advapi32.dll")
	procSetInfo       = kernel32Permanent.NewProc("SetFileInformationByHandle")
	procFinalPath     = kernel32Permanent.NewProc("GetFinalPathNameByHandleW")
	procVolumeInfo    = kernel32Permanent.NewProc("GetVolumeInformationW")
	procGetCurrent    = kernel32Permanent.NewProc("GetCurrentProcess")
	procOpenToken     = advapi32Permanent.NewProc("OpenProcessToken")
	procTokenInfo     = advapi32Permanent.NewProc("GetTokenInformation")
)

type tokenElevation struct {
	TokenIsElevated uint32
}

type windowsExecutor struct {
	trustedRoot string
	initErr     error
}

func NewNativeExecutor() Executor {
	rule, ok := cleanup.RuleByID(RuleID)
	if !ok {
		return windowsExecutor{initErr: fmt.Errorf("crash-dump cleanup rule is unavailable")}
	}
	root, err := rule.ResolveRoot()
	if err == nil {
		root, err = storage.ValidateRoot(root)
	}
	return windowsExecutor{trustedRoot: root, initErr: err}
}

func (e windowsExecutor) Available() error {
	if os.Getenv("STACKBROOM_ENABLE_PERMANENT_DELETE") != "1" {
		return ErrDisabled
	}
	if e.initErr != nil || e.trustedRoot == "" {
		return fmt.Errorf("trusted crash-dump scope is unavailable")
	}
	elevated, err := processElevated()
	if err != nil {
		return fmt.Errorf("elevation state could not be checked: %w", err)
	}
	if elevated {
		return ErrElevated
	}
	return nil
}

func (e windowsExecutor) Execute(ctx context.Context, targets []execution.Target, progress func(Progress)) (Result, error) {
	requested := len(targets)
	if err := e.Available(); err != nil {
		return Result{State: StateFailed, Requested: requested, ReasonCode: "execution_disabled", Message: err.Error()}, err
	}
	if len(targets) == 0 {
		return Result{State: StateFailed, Requested: 0, ReasonCode: "no_targets", Message: "no files were selected"}, fmt.Errorf("no files selected")
	}
	items := make([]ItemResult, 0, len(targets))
	for index, target := range targets {
		if ctx.Err() != nil {
			return Result{State: StateCancelled, Requested: requested, Items: items, Aborted: true, ReasonCode: "cancelled"}, ctx.Err()
		}
		item := deleteOne(ctx, e.trustedRoot, target)
		items = append(items, item)
		if progress != nil {
			progress(Progress{State: StateExecuting, TargetID: target.ID, Completed: index + 1, Total: len(targets)})
		}
	}
	return Result{State: StateVerifying, Requested: requested, Items: items}, nil
}

type fileDispositionInfoData struct {
	DeleteFile byte
}

func deleteOne(ctx context.Context, trustedRoot string, target execution.Target) ItemResult {
	item := ItemResult{
		TargetID: target.ID, Status: ItemUnknown, LogicalBytes: target.File.LogicalBytes,
		AllocatedBytes: target.File.AllocatedBytes, AllocatedKnown: target.File.AllocatedKnown,
		RecoveryAvailable: false,
	}
	if ctx.Err() != nil {
		item.Status, item.ReasonCode, item.Message = ItemCancelled, "cancelled", "the operation was cancelled"
		return item
	}
	if trustedRoot == "" || !target.File.RootEqual(trustedRoot) || !pathWithinRoot(trustedRoot, target.File.Path) {
		item.Status, item.ReasonCode, item.Message = ItemRefused, "scope", "the target is outside the trusted crash-dump root"
		return item
	}
	if !strings.EqualFold(filepath.Ext(target.File.Name), ".dmp") || !isNTFS(target.File.Path) {
		item.Status, item.ReasonCode, item.Message = ItemRefused, "unsupported_filesystem", "only .dmp files on NTFS are supported"
		return item
	}
	if target.File.ModTime.IsZero() || time.Since(target.File.ModTime) < 7*24*time.Hour {
		item.Status, item.ReasonCode, item.Message = ItemChanged, "age", "the file is not old enough for crash-dump cleanup"
		return item
	}
	if err := storage.ValidateCleanupTarget(&target.File); err != nil {
		code := storage.ValidateTargetErrorCode(err)
		item.Status, item.ReasonCode, item.Message = validationStatus(code), code, "the file changed before cleanup"
		return item
	}
	release, err := storage.GuardCleanupAncestors(target.File)
	if err != nil {
		item.Status, item.ReasonCode, item.Message = ItemRefused, "ancestor_guard", "the file's folder could not be protected"
		return item
	}
	defer release()
	handle, err := openExclusive(target.File.Path)
	if err != nil {
		item.Status, item.ReasonCode, item.Message = classifyError(err, "open")
		return item
	}
	closed := false
	defer func() {
		if !closed {
			_ = syscall.CloseHandle(handle)
		}
	}()
	if item = verifyHandle(handle, trustedRoot, target); item.Status != ItemValidated {
		return item
	}
	if ctx.Err() != nil {
		item.Status, item.ReasonCode, item.Message = ItemCancelled, "cancelled", "the operation was cancelled"
		return item
	}
	disposition := fileDispositionInfoData{DeleteFile: 1}
	r, _, callErr := procSetInfo.Call(
		uintptr(handle), fileDispositionInfo,
		uintptr(unsafe.Pointer(&disposition)), unsafe.Sizeof(disposition),
	)
	if r == 0 {
		item.Status, item.ReasonCode, item.Message = classifyError(callErr, "disposition")
		return item
	}
	if err := syscall.CloseHandle(handle); err != nil {
		closed = true
		item.Status, item.ReasonCode, item.Message = ItemUnknown, "close_failed", "the file handle could not be closed"
		return item
	}
	closed = true
	if _, err := os.Lstat(target.File.Path); err == nil {
		item.Status, item.ReasonCode, item.Message = ItemStillPresent, "still_present", "the file is still present after the operation"
		return item
	} else if !os.IsNotExist(err) {
		item.Status, item.ReasonCode, item.Message = ItemUnknown, "verification_failed", "the final filesystem state could not be checked"
		return item
	}
	item.Status, item.ReasonCode, item.Message = ItemDeleted, "deleted", "the file was permanently deleted"
	item.SpaceReclaimed = target.File.AllocatedBytes
	item.SpaceReclaimedKnown = false
	item.Verified = true
	return item
}

func openExclusive(path string) (syscall.Handle, error) {
	pathPointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	// No sharing is deliberate: an existing writer, mapper or delete-capable
	// handle makes the open fail instead of allowing a delayed or forced delete.
	return syscall.CreateFile(
		pathPointer, deleteAccess|fileReadAttributes, 0, nil, openExisting,
		fileFlagOpenReparsePoint, 0,
	)
}

func verifyHandle(handle syscall.Handle, trustedRoot string, target execution.Target) ItemResult {
	item := ItemResult{
		TargetID: target.ID, Status: ItemUnknown, RecoveryAvailable: false,
		LogicalBytes: target.File.LogicalBytes, AllocatedBytes: target.File.AllocatedBytes,
		AllocatedKnown: target.File.AllocatedKnown,
	}
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil {
		item.Status, item.ReasonCode, item.Message = ItemUnknown, "identity_read_failed", "the file identity could not be read"
		return item
	}
	attributes := info.FileAttributes
	if attributes&fileAttributeDirectory != 0 {
		item.Status, item.ReasonCode, item.Message = ItemRefused, "directory", "directories are never deleted"
		return item
	}
	if attributes&(fileAttributeReparse|fileAttributeOffline|fileAttributeRecallOpen|fileAttributeRecallData) != 0 {
		item.Status, item.ReasonCode, item.Message = ItemRefused, "special_file", "links and cloud files are not supported"
		return item
	}
	if attributes&fileAttributeHidden != 0 || attributes&fileAttributeSystem != 0 {
		item.Status, item.ReasonCode, item.Message = ItemRefused, "protected_file", "hidden and system files are not supported"
		return item
	}
	if attributes&fileAttributeReadOnly != 0 {
		item.Status, item.ReasonCode, item.Message = ItemRefused, "read_only", "read-only files are not deleted"
		return item
	}
	if attributes&fileAttributeSparse != 0 {
		item.Status, item.ReasonCode, item.Message = ItemRefused, "sparse", "sparse files are not supported"
		return item
	}
	if info.NumberOfLinks != 1 {
		item.Status, item.ReasonCode, item.Message = ItemRefused, "hard_link", "files with multiple links are not deleted"
		return item
	}
	size := uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow)
	lastWrite := time.Unix(0, info.LastWriteTime.Nanoseconds())
	fileID := fmt.Sprintf("%08x:%08x%08x", info.VolumeSerialNumber, info.FileIndexHigh, info.FileIndexLow)
	if size != target.File.LogicalBytes || !lastWrite.Equal(target.File.ModTime) || fileID != target.File.FileID {
		item.Status, item.ReasonCode, item.Message = ItemChanged, "identity_changed", "the file changed since the scan"
		return item
	}
	finalPath, err := finalPath(handle)
	if err != nil || !pathWithinRoot(trustedRoot, finalPath) {
		item.Status, item.ReasonCode, item.Message = ItemChanged, "scope_changed", "the open file is outside the approved crash-dump folder"
		return item
	}
	item.Status = ItemValidated
	return item
}

// finalPathBufferUsable mirrors GetFinalPathNameByHandleW: a result at or above
// the supplied capacity means the required terminator cannot fit.
func finalPathBufferUsable(length uintptr, capacity int) bool {
	return length > 0 && length < uintptr(capacity)
}

func finalPath(handle syscall.Handle) (string, error) {
	buffer := make([]uint16, 2048)
	r, _, callErr := procFinalPath.Call(
		uintptr(handle), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0,
	)
	if r == 0 {
		return "", callErr
	}
	if !finalPathBufferUsable(r, len(buffer)) {
		return "", fmt.Errorf("path is too long")
	}
	path := syscall.UTF16ToString(buffer[:r])
	if strings.HasPrefix(path, `\\?\UNC\`) {
		return "", fmt.Errorf("UNC paths are unsupported")
	}
	path = strings.TrimPrefix(path, `\\?\`)
	return filepath.Clean(path), nil
}

func pathWithinRoot(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return false
	}
	return true
}

func isNTFS(path string) bool {
	volume := filepath.VolumeName(path)
	if len(volume) != 2 || volume[1] != ':' {
		return false
	}
	root, err := syscall.UTF16PtrFromString(volume + string(filepath.Separator))
	if err != nil {
		return false
	}
	var fileSystem [64]uint16
	r, _, _ := procVolumeInfo.Call(
		uintptr(unsafe.Pointer(root)), 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&fileSystem[0])), uintptr(len(fileSystem)),
	)
	if r == 0 {
		return false
	}
	return strings.EqualFold(syscall.UTF16ToString(fileSystem[:]), "NTFS")
}

func processElevated() (bool, error) {
	current, _, _ := procGetCurrent.Call()
	var token syscall.Handle
	r, _, callErr := procOpenToken.Call(current, 0x0008, uintptr(unsafe.Pointer(&token))) // TOKEN_QUERY
	if r == 0 {
		return false, callErr
	}
	defer syscall.CloseHandle(token)
	var info tokenElevation
	var returned uint32
	r, _, callErr = procTokenInfo.Call(
		uintptr(token), 20, // TokenElevation
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info),
		uintptr(unsafe.Pointer(&returned)),
	)
	if r == 0 {
		return false, callErr
	}
	return info.TokenIsElevated != 0, nil
}

func validationStatus(code string) ItemStatus {
	switch code {
	case "hidden", "system", "reparse", "placeholder", "hard_link", "not_regular", "scope":
		return ItemRefused
	default:
		return ItemChanged
	}
}

func classifyError(err error, where string) (ItemStatus, string, string) {
	errno, ok := err.(syscall.Errno)
	if !ok {
		return ItemUnknown, "operation_failed", "the file operation could not be completed"
	}
	switch errno {
	case errorSharingViolation:
		return ItemRefused, "in_use", "the file is currently in use"
	case errorAccessDenied:
		return ItemRefused, "access_denied", "Windows did not allow this file to be deleted"
	case errorCannotDelete:
		return ItemRefused, "cannot_delete", "Windows could not delete this file"
	default:
		if where == "open" {
			return ItemRefused, "open_failed", "the file could not be opened for deletion"
		}
		return ItemUnknown, "disposition_failed", "the delete request could not be completed"
	}
}
