package cleanup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTempFile(t *testing.T, path string, age time.Duration, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(make([]byte, size)), 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverTempRuleAgeAndHardLinkExclusions(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TEMP", root)
	t.Setenv("TMP", root)
	oldPath := filepath.Join(root, "old.tmp")
	writeTempFile(t, oldPath, 30*24*time.Hour, 100)
	writeTempFile(t, filepath.Join(root, "new.tmp"), time.Hour, 100)
	linkPath := filepath.Join(root, "old-link.tmp")
	if err := os.Link(oldPath, linkPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	rule, ok := RuleByID("user-temp-old")
	if !ok {
		t.Fatal("rule missing")
	}
	discovery, err := Discover(context.Background(), rule, 100, nil, time.Now)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(discovery.Candidates) != 0 {
		t.Errorf("hard-linked files must not be candidates: %+v", discovery.Candidates)
	}
	if discovery.ExcludedCounts["newer than 14 days"] == 0 || discovery.ExcludedCounts["hard-linked file"] == 0 {
		t.Errorf("exclusion counts = %+v", discovery.ExcludedCounts)
	}
}

func TestDiscoverTempRuleEligibleFileAndPreview(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TEMP", root)
	t.Setenv("TMP", root)
	path := filepath.Join(root, "old.tmp")
	writeTempFile(t, path, 30*24*time.Hour, 300)
	rule, _ := RuleByID("user-temp-old")
	discovery, err := Discover(context.Background(), rule, 100, nil, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.Candidates) != 1 {
		t.Fatalf("candidates = %+v", discovery.Candidates)
	}
	preview, err := BuildPreview(discovery, []string{discovery.Candidates[0].ID}, time.Now(), 10*time.Minute)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Selected) != 1 || preview.DeletionMode != "recycle" || preview.RecoverableBytes != 300 {
		t.Errorf("preview = %+v", preview)
	}
	if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildPreview(discovery, []string{discovery.Candidates[0].ID}, time.Now(), 10*time.Minute); err == nil {
		t.Error("changed candidate was accepted in a preview")
	}
}

func TestDiscoverCrashDumpRuleExtension(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	writeTempFile(t, filepath.Join(root, "CrashDumps", "old.dmp"), 30*24*time.Hour, 100)
	writeTempFile(t, filepath.Join(root, "CrashDumps", "old.txt"), 30*24*time.Hour, 100)
	rule, _ := RuleByID("crash-dumps-old")
	discovery, err := Discover(context.Background(), rule, 100, nil, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.Candidates) != 1 || discovery.Candidates[0].File.Name != "old.dmp" {
		t.Errorf("crash dump candidates = %+v", discovery.Candidates)
	}
	if discovery.ExcludedCounts["extension excluded by rule"] == 0 {
		t.Errorf("extension exclusion = %+v", discovery.ExcludedCounts)
	}
}
