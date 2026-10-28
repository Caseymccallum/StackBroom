//go:build windows

package storage

import (
	"context"
	"os"
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
	if excluded.TotalFiles != 0 || excluded.SkippedCount == 0 || excluded.Coverage.HiddenExcluded != 1 || excluded.Coverage.Exhaustive {
		t.Errorf("hidden file was not excluded: %+v", excluded)
	}
	included := scanFixture(t, root, func(request *Request) { request.IncludeHidden = true })
	if included.TotalFiles != 1 || included.TotalLogical != 32 {
		t.Errorf("hidden file was not included when requested: %+v", included)
	}
}

func TestOfflinePlaceholderCannotBeHashed(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cloud-placeholder.bin")
	writeFile(t, path, 8192, 'c')
	result := scanFixture(t, root, nil)
	if len(result.Files) != 1 {
		t.Fatal("fixture file was not indexed")
	}
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	attributes, err := syscall.GetFileAttributes(pointer)
	if err != nil {
		t.Skipf("attributes unavailable: %v", err)
	}
	if err := syscall.SetFileAttributes(pointer, attributes|0x00001000); err != nil {
		t.Skipf("cannot mark offline placeholder: %v", err)
	}
	t.Cleanup(func() { _ = syscall.SetFileAttributes(pointer, attributes) })
	if _, ok := hashFile(context.Background(), result.Files[0], 0); ok {
		t.Error("offline placeholder was opened for hashing")
	}
}

func TestReplacingFileWithJunctionPreventsVerifiedRead(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left.bin")
	right := filepath.Join(root, "right.bin")
	writeFile(t, left, 8192, 'x')
	writeFile(t, right, 8192, 'x')
	result := scanFixture(t, root, nil)
	if len(result.Files) != 2 {
		t.Fatalf("files = %d", len(result.Files))
	}
	target := t.TempDir()
	if err := os.Remove(left); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, left, target)
	record := result.Files[0]
	if fileStillMatches(record) {
		t.Error("replaced junction passed post-hash identity check")
	}
	if _, ok := hashFile(context.Background(), record, 0); ok {
		t.Error("replaced junction was read as file content")
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
