//go:build windows

package storage

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestAncestorGuardDeniesDeleteAccessWithoutMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fixture.dmp")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	request := DefaultRequest(root)
	request.MinDuplicateSize = ^uint64(0)
	scan, err := NewScanner().Scan(context.Background(), request, nil)
	if err != nil || len(scan.Files) != 1 {
		t.Fatal(err)
	}
	release, err := GuardCleanupAncestors(*scan.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ptr, _ := syscall.UTF16PtrFromString(root)
	// Request access only; do not rename, remove, or mark anything for deletion.
	handle, err := syscall.CreateFile(ptr, 0x00010000, fileShareRead|fileShareWrite|fileShareDelete, nil, openExisting, fileFlagBackupSemantics, 0)
	if err == nil {
		syscall.CloseHandle(handle)
		t.Fatal("guard allowed DELETE access to ancestor")
	}
	if err != syscall.Errno(32) { // ERROR_SHARING_VIOLATION
		t.Fatalf("expected sharing denial, got %v", err)
	}
}
