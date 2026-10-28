//go:build windows

package storage

import (
	"context"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func makeJunction(t *testing.T, link, target string) {
	t.Helper()
	command := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("junction unavailable: %v (%s)", err, output)
	}
}

func TestJunctionIsReportedAndNotTraversed(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "outside.bin"), 256, 'x')
	makeJunction(t, filepath.Join(root, "junction"), outside)

	result := scanFixture(t, root, nil)
	if result.TotalLogical != 0 {
		t.Errorf("junction target traversed: %d bytes", result.TotalLogical)
	}
	if result.SkippedCount == 0 {
		t.Error("junction was not reported as skipped")
	}
}

func TestHiddenFilesAreExcludedByDefault(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "hidden.txt")
	writeFile(t, path, 32, 'h')
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	attributes, err := syscall.GetFileAttributes(pointer)
	if err != nil {
		t.Skipf("attributes unavailable: %v", err)
	}
	if err := syscall.SetFileAttributes(pointer, attributes|0x00000002); err != nil {
		t.Skipf("cannot mark hidden: %v", err)
	}
	t.Cleanup(func() { _ = syscall.SetFileAttributes(pointer, attributes) })

	excluded := scanFixture(t, root, nil)
	if excluded.TotalFiles != 0 || excluded.SkippedCount == 0 {
		t.Errorf("hidden file was not excluded: %+v", excluded)
	}
	included := scanFixture(t, root, func(request *Request) { request.IncludeHidden = true })
	if included.TotalFiles != 1 || included.TotalLogical != 32 {
		t.Errorf("hidden file was not included when requested: %+v", included)
	}
}

func TestJunctionRootIsRejected(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "junction-root")
	makeJunction(t, link, target)
	if _, err := NewScanner().Scan(context.Background(), DefaultRequest(link), nil); err == nil {
		t.Error("junction root accepted")
	}
}
