package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path string, size int, fill byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat(string(fill), size)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scanFixture(t *testing.T, root string, mutate func(*Request)) *Result {
	t.Helper()
	request := DefaultRequest(root)
	request.MinDuplicateSize = 1
	if mutate != nil {
		mutate(&request)
	}
	result, err := NewScanner().Scan(context.Background(), request, nil)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	return result
}

func TestScanAggregatesTreeTypesAndDuplicates(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "one.txt"), 100, 'a')
	writeFile(t, filepath.Join(root, "a", "two.log"), 200, 'b')
	writeFile(t, filepath.Join(root, "b", "one-copy.txt"), 100, 'a')
	writeFile(t, filepath.Join(root, "b", "nested", "three.bin"), 300, 'c')

	result := scanFixture(t, root, nil)
	if result.Partial {
		t.Fatalf("unexpected partial scan: %s", result.PartialReason)
	}
	if result.TotalFiles != 4 || result.TotalLogical != 700 {
		t.Fatalf("totals = files %d bytes %d", result.TotalFiles, result.TotalLogical)
	}
	if len(result.Duplicates) != 1 || len(result.Duplicates[0].Files) != 2 {
		t.Fatalf("duplicate groups = %+v", result.Duplicates)
	}
	if result.Duplicates[0].Size != 100 || result.Duplicates[0].Hash == "" {
		t.Errorf("duplicate evidence = %+v", result.Duplicates[0])
	}
	textBytes := uint64(0)
	for _, stat := range result.Types {
		if stat.Extension == "txt" {
			textBytes = stat.LogicalBytes
		}
	}
	if textBytes != 200 {
		t.Errorf("txt type bytes = %d, types = %+v", textBytes, result.Types)
	}
	if len(result.LargestFiles) == 0 || result.LargestFiles[0].LogicalBytes != 300 {
		t.Errorf("largest files = %+v", result.LargestFiles)
	}
	if len(result.LargestFolders) == 0 || result.LargestFolders[0].Name != "b" {
		t.Errorf("largest folders = %+v", result.LargestFolders)
	}
	if !result.Coverage.Exhaustive {
		t.Errorf("unexpected coverage = %+v", result.Coverage)
	}
	if result.DuplicateCoverage.Enumerated != 4 || result.DuplicateCoverage.Eligible != 4 || result.DuplicateCoverage.Verified != 2 {
		t.Errorf("duplicate coverage = %+v", result.DuplicateCoverage)
	}
}

func TestScanDoesNotCountHardLinksTwiceAndDoesNotCallThemCopies(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "original.bin")
	writeFile(t, original, 4096, 'x')
	link := filepath.Join(root, "nested", "link.bin")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(original, link); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	result := scanFixture(t, root, nil)
	if result.TotalLogical != 4096 {
		t.Errorf("hard-linked logical bytes = %d, want 4096", result.TotalLogical)
	}
	accounted := 0
	for _, file := range result.Files {
		if file.Accounted {
			accounted++
		}
	}
	if accounted != 1 {
		t.Errorf("accounted hard-link entries = %d, want 1", accounted)
	}
	if len(result.Duplicates) != 0 {
		t.Errorf("hard links must not be reported as independent duplicates: %+v", result.Duplicates)
	}
}

func TestScanDoesNotFollowDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "outside.txt"), 500, 'o')
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	result := scanFixture(t, root, nil)
	if result.TotalLogical != 0 {
		t.Errorf("symlink target was traversed: %d bytes", result.TotalLogical)
	}
	if result.SkippedCount == 0 {
		t.Error("reparse point was not reported as skipped")
	}
}

func TestScanCancellationAndEntryLimitArePartial(t *testing.T) {
	root := t.TempDir()
	for index := 0; index < 5; index++ {
		writeFile(t, filepath.Join(root, "file"+string(rune('a'+index))+".txt"), 10, 'x')
	}
	request := DefaultRequest(root)
	request.MaxEntries = 2
	result, err := NewScanner().Scan(context.Background(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Partial || !strings.Contains(result.PartialReason, "limit") {
		t.Errorf("entry limit result = partial %v reason %q", result.Partial, result.PartialReason)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = NewScanner().Scan(ctx, DefaultRequest(root), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Partial || result.PartialReason != "cancelled" {
		t.Errorf("cancelled result = partial %v reason %q", result.Partial, result.PartialReason)
	}
	if !result.DuplicateCoverage.Truncated {
		t.Errorf("cancelled duplicate analysis was reported complete: %+v", result.DuplicateCoverage)
	}
}

func TestScanRejectsMissingAndReparseRoots(t *testing.T) {
	if _, err := NewScanner().Scan(context.Background(), DefaultRequest(filepath.Join(t.TempDir(), "missing")), nil); err == nil {
		t.Error("missing root accepted")
	}
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := NewScanner().Scan(context.Background(), DefaultRequest(link), nil); err == nil {
		t.Error("reparse root accepted")
	}
}

func TestScanHandlesUnicodeAndDeepNames(t *testing.T) {
	root := t.TempDir()
	deep := root
	for index := 0; index < 8; index++ {
		deep = filepath.Join(deep, "level-"+string(rune('a'+index)))
	}
	writeFile(t, filepath.Join(deep, "café-文件.txt"), 42, 'u')
	result := scanFixture(t, root, nil)
	if result.TotalFiles != 1 || result.TotalLogical != 42 {
		t.Fatalf("unicode scan totals = %+v", result)
	}
}

func TestDuplicateCandidateCapIsReported(t *testing.T) {
	root := t.TempDir()
	for index := 0; index < 4; index++ {
		writeFile(t, filepath.Join(root, fmt.Sprintf("copy-%d.bin", index)), 100, 'd')
	}
	result := scanFixture(t, root, func(request *Request) { request.MaxDuplicateFiles = 2 })
	if result.DuplicateCoverage.Eligible != 4 || result.DuplicateCoverage.CandidateConsidered != 2 || !result.DuplicateCoverage.Truncated {
		t.Errorf("duplicate cap coverage = %+v", result.DuplicateCoverage)
	}
	if !result.Coverage.Exhaustive {
		t.Errorf("traversal coverage unexpectedly incomplete: %+v", result.Coverage)
	}
}

func TestPartialHashMatchStillRequiresFullHash(t *testing.T) {
	root := t.TempDir()
	prefix := strings.Repeat("p", 70*1024)
	writeFile(t, filepath.Join(root, "first.bin"), len(prefix), 'p')
	writeFile(t, filepath.Join(root, "second.bin"), len(prefix), 'p')
	firstPath := filepath.Join(root, "first.bin")
	secondPath := filepath.Join(root, "second.bin")
	suffix := strings.Repeat("different", 100)
	first, err := os.OpenFile(firstPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = first.WriteString(suffix)
	first.Close()
	second, err := os.OpenFile(secondPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = second.WriteString(strings.Repeat("other", 180))
	second.Close()

	result := scanFixture(t, root, nil)
	if len(result.Duplicates) != 0 {
		t.Errorf("partial-prefix collision accepted as duplicate: %+v", result.Duplicates)
	}
}

func TestFileRecordOutsideRootIsRejected(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	path := filepath.Join(outside, "secret.txt")
	writeFile(t, path, 10, 's')
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	record := &FileRecord{Root: root, Path: path, LogicalBytes: 10, ModTime: info.ModTime()}
	if fileStillMatches(record) {
		t.Error("out-of-root file passed identity check")
	}
	if _, ok := hashFile(context.Background(), record, 0); ok {
		t.Error("out-of-root file was read")
	}
}

func TestChangedFileIsRejectedAfterHashing(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "changing.txt")
	writeFile(t, path, 10, 'a')
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	record := &FileRecord{Path: path, LogicalBytes: 10, ModTime: info.ModTime(), FileID: ""}
	if !fileStillMatches(record) {
		t.Fatal("stable file rejected")
	}
	time.Sleep(20 * time.Millisecond)
	writeFile(t, path, 11, 'b')
	if fileStillMatches(record) {
		t.Error("changed file accepted")
	}
}
