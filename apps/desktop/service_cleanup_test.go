package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"github.com/Caseymccallum/StackBroom/internal/storage"
)

func TestCleanupServiceMutationSurfaceIsGated(t *testing.T) {
	service := NewCleanupService()
	if len(service.ListCleanupRules()) < 2 {
		t.Fatal("curated rule catalogue is empty")
	}
	for _, method := range []string{"DeleteFiles", "MoveToRecycleBin", "CleanupFiles"} {
		if reflect.ValueOf(service).MethodByName(method).IsValid() {
			t.Errorf("unapproved mutation method %s exists", method)
		}
	}
	t.Setenv("STACKBROOM_ENABLE_CLEANUP_EXECUTION", "1")
	if _, err := service.ConfirmCleanup("anything"); err == nil {
		t.Error("environment variable enabled cleanup execution in an ordinary build")
	}
}

func TestPermanentConfirmationRefusedInOrdinaryBuild(t *testing.T) {
	service := NewCleanupService()
	t.Setenv("STACKBROOM_ENABLE_PERMANENT_DELETE", "1")
	if _, err := service.ConfirmPermanentCleanup("missing"); err == nil {
		t.Fatal("ordinary build accepted a permanent cleanup confirmation")
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
	rule, _ := cleanup.RuleByID("user-temp-old")
	trustedRoot, err := rule.ResolveRoot()
	if err != nil {
		t.Skipf("trusted temp scope unavailable: %v", err)
	}
	root := trustedRoot
	fixtureDir, err := os.MkdirTemp(root, "stackbroom-m3b-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fixtureDir)
	path := filepath.Join(fixtureDir, fmt.Sprintf("old-%d.tmp", time.Now().UnixNano()))
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	request := storage.DefaultRequest(fixtureDir)
	request.MinDuplicateSize = ^uint64(0)
	scan, err := storage.NewScanner().Scan(context.Background(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	var record storage.FileRecord
	for _, file := range scan.Files {
		if file.Path == path {
			record = *file
			break
		}
	}
	if record.FileID == "" {
		t.Fatalf("fixture file was not indexed: %+v", scan.Files)
	}
	record.Root = root
	now := time.Now()
	service := NewCleanupService()
	service.now = func() time.Time { return now }
	service.discoverFn = func(ctx context.Context, rule cleanup.Rule, max int, progress func(cleanup.Progress), clock func() time.Time) (*cleanup.Discovery, error) {
		return &cleanup.Discovery{
			Rule: rule, Root: root, ScannedFiles: 1, Eligible: 1,
			Candidates:   []cleanup.Candidate{{ID: "c1", RelativePath: "old.tmp", File: record}},
			TotalLogical: record.LogicalBytes, TotalAllocated: record.AllocatedBytes,
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
