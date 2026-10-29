//go:build windows && cleanup_execution

package permanent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/storage"
)

func TestPermanentNativeGateRefusesWithoutEnvironment(t *testing.T) {
	t.Setenv("STACKBROOM_ENABLE_PERMANENT_DELETE", "0")
	executor := NewNativeExecutor()
	if err := executor.Available(); err == nil {
		t.Fatal("native executor advertised availability without the explicit gate")
	}
	result, err := executor.Execute(context.Background(), nil, nil)
	if err == nil || result.State != StateFailed {
		t.Fatalf("result = %+v err=%v", result, err)
	}
	elevated, elevationErr := processElevated()
	if elevationErr != nil {
		t.Fatal(elevationErr)
	}
	if elevated {
		t.Skip("test process is elevated; the delete gate correctly refuses it")
	}
	t.Setenv("STACKBROOM_ENABLE_PERMANENT_DELETE", "1")
	result, err = executor.Execute(context.Background(), nil, nil)
	if err == nil || result.ReasonCode != "no_targets" || result.State != StateFailed {
		t.Fatalf("empty target result = %+v err=%v", result, err)
	}
}

func TestPermanentPathWithinRoot(t *testing.T) {
	root := t.TempDir()
	if !pathWithinRoot(root, filepath.Join(root, "old.dmp")) {
		t.Fatal("direct child was rejected")
	}
	if pathWithinRoot(root, filepath.Join(filepath.Dir(root), "escape.dmp")) {
		t.Fatal("sibling path was accepted")
	}
	if pathWithinRoot(root, root) {
		t.Fatal("root itself was accepted")
	}
}

func TestPermanentFinalPathBufferBoundary(t *testing.T) {
	if finalPathBufferUsable(0, 2048) || finalPathBufferUsable(2048, 2048) || finalPathBufferUsable(2049, 2048) {
		t.Fatal("full or overflowing final-path buffer was accepted")
	}
	if !finalPathBufferUsable(1, 2048) || !finalPathBufferUsable(2047, 2048) {
		t.Fatal("usable final-path buffer was rejected")
	}
}

func TestPermanentClassifyErrors(t *testing.T) {
	if status, code, _ := classifyError(syscall.Errno(32), "open"); status != ItemRefused || code != "in_use" {
		t.Fatalf("sharing = %s %s", status, code)
	}
	if status, code, _ := classifyError(syscall.Errno(5), "disposition"); status != ItemRefused || code != "access_denied" {
		t.Fatalf("access = %s %s", status, code)
	}
	if status, code, _ := classifyError(syscall.Errno(600), "disposition"); status != ItemRefused || code != "cannot_delete" {
		t.Fatalf("cannot delete = %s %s", status, code)
	}
	if status, code, _ := classifyError(errors.New("other"), "disposition"); status != ItemUnknown || code != "operation_failed" {
		t.Fatalf("other = %s %s", status, code)
	}
}

func TestPermanentExclusiveOpenRefusesAnotherHandle(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	other, err := os.Open(f.target.File.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	handle, err := openExclusive(f.target.File.Path)
	if err == nil {
		_ = syscall.CloseHandle(handle)
		t.Skip("platform sharing allowed a second delete-capable open")
	}
	if err != syscall.Errno(32) && err != syscall.Errno(5) {
		t.Fatalf("unexpected lock refusal: %v", err)
	}
}

func TestPermanentOpenAndVerifyHandleDoesNotDelete(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	release, err := storage.GuardCleanupAncestors(f.target.File)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	handle, err := openExclusive(f.target.File.Path)
	if err != nil {
		t.Fatal(err)
	}
	item := verifyHandle(handle, f.root, f.target)
	if item.Status != ItemValidated {
		t.Fatalf("handle verification = %+v", item)
	}
	if item.LogicalBytes != f.target.File.LogicalBytes || item.AllocatedBytes != f.target.File.AllocatedBytes || item.AllocatedKnown != f.target.File.AllocatedKnown {
		t.Fatalf("verified metadata was not preserved: %+v", item)
	}
	if err := syscall.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(f.target.File.Path); err != nil {
		t.Fatalf("non-mutating open/verify removed the fixture: %v", err)
	}
}
