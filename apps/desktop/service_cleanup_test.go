package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"github.com/Caseymccallum/StackBroom/internal/storage"
)

func TestCleanupServiceHasOnlyDiscoveryAndPreviewMethods(t *testing.T) {
	service := NewCleanupService()
	if len(service.ListCleanupRules()) < 2 {
		t.Fatal("curated rule catalogue is empty")
	}
	for _, method := range []string{"ConfirmCleanup", "DeleteFiles", "MoveToRecycleBin", "CleanupFiles"} {
		if reflect.ValueOf(service).MethodByName(method).IsValid() {
			t.Errorf("mutation-capable method %s exists", method)
		}
	}
}

func TestCleanupServiceCancellationIsReported(t *testing.T) {
	rule, _ := cleanup.RuleByID("user-temp-old")
	service := NewCleanupService()
	service.discoverFn = func(ctx context.Context, rule cleanup.Rule, max int, progress func(cleanup.Progress), clock func() time.Time) (*cleanup.Discovery, error) {
		<-ctx.Done()
		return &cleanup.Discovery{Rule: rule, Partial: true, PartialReason: "cancelled"}, nil
	}
	discoveryID, err := service.StartCleanupDiscovery(rule.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !service.CancelCleanupDiscovery(discoveryID) {
		t.Fatal("running discovery was not cancellable")
	}
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		view, err := service.GetCleanupDiscovery(discoveryID)
		if err != nil {
			t.Fatal(err)
		}
		if view.Status != "running" {
			if view.Status != "cancelled" || !view.Partial {
				t.Errorf("cancelled discovery = %+v", view)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("cancelled discovery did not finish")
}

func TestCleanupServicePreviewExpires(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "old.tmp")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	rule, _ := cleanup.RuleByID("user-temp-old")
	service := NewCleanupService()
	service.now = func() time.Time { return now }
	service.discoverFn = func(ctx context.Context, rule cleanup.Rule, max int, progress func(cleanup.Progress), clock func() time.Time) (*cleanup.Discovery, error) {
		return &cleanup.Discovery{
			Rule: rule, Root: root, ScannedFiles: 1, Eligible: 1,
			Candidates: []cleanup.Candidate{{ID: "c1", RelativePath: "old.tmp", File: storage.FileRecord{
				Root: root, Path: path, RelativePath: "old.tmp", Name: "old.tmp",
				LogicalBytes: uint64(info.Size()), AllocatedBytes: uint64(info.Size()), AllocatedKnown: true,
				ModTime: info.ModTime(),
			}}},
			TotalLogical: uint64(info.Size()), TotalAllocated: uint64(info.Size()),
		}, nil
	}
	discoveryID, err := service.StartCleanupDiscovery(rule.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	var view CleanupDiscoveryView
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		view, err = service.GetCleanupDiscovery(discoveryID)
		if err != nil {
			t.Fatal(err)
		}
		if view.Status != "running" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if view.Status != "completed" || len(view.Candidates) != 1 {
		t.Fatalf("discovery = %+v", view)
	}
	preview, err := service.CreateCleanupPreview(discoveryID, []string{"c1"})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.FileCount != 1 || preview.DeletionMode != "recycle" {
		t.Errorf("preview = %+v", preview)
	}
	if _, err := service.CreateCleanupPreview(discoveryID, []string{"..\\arbitrary-path"}); err == nil {
		t.Error("an arbitrary path was accepted as a preview target")
	}
	now = now.Add(11 * time.Minute)
	if _, err := service.GetCleanupPreview(preview.PreviewID); err == nil {
		t.Error("expired preview was returned")
	}
}
