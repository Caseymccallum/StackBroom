package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestScanSyntheticPerformance is opt-in so normal test runs stay bounded.
// It uses only a disposable temp tree and never the owner's profile.
func TestScanSyntheticPerformance(t *testing.T) {
	if os.Getenv("STACKBROOM_PERF") != "1" {
		t.Skip("set STACKBROOM_PERF=1 to run the synthetic storage performance test")
	}
	root := t.TempDir()
	const folders = 50
	const filesPerFolder = 100
	content := []byte("0123456789abcdef0123456789abcdef")
	for folderIndex := 0; folderIndex < folders; folderIndex++ {
		folder := filepath.Join(root, fmt.Sprintf("folder-%02d", folderIndex))
		if err := os.MkdirAll(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		for fileIndex := 0; fileIndex < filesPerFolder; fileIndex++ {
			if err := os.WriteFile(filepath.Join(folder, fmt.Sprintf("file-%03d.txt", fileIndex)), content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	result, err := NewScanner().Scan(context.Background(), DefaultRequest(root), nil)
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scan files=%d dirs=%d elapsed=%s heap_alloc_delta=%dKB total_alloc_delta=%dKB",
		result.TotalFiles, result.TotalDirectories, elapsed,
		(after.HeapAlloc-before.HeapAlloc)/1024,
		(after.TotalAlloc-before.TotalAlloc)/1024,
	)
	if result.TotalFiles != folders*filesPerFolder {
		t.Errorf("files = %d", result.TotalFiles)
	}
}
