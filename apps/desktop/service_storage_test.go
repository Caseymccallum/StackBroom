package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/storage"
)

func storageFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	duplicate := strings.Repeat("s", 8192)
	files := map[string]string{
		"docs/readme.txt":   duplicate,
		"backup/readme.txt": duplicate,
		"media/large.bin":   strings.Repeat("x", 8192),
		"media/small.log":   "different",
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func newTestStorageService(t *testing.T, root string) *StorageService {
	t.Helper()
	service := NewStorageService()
	service.selectDir = func() (string, error) { return root, nil }
	return service
}

func waitForStorageScan(t *testing.T, service *StorageService, scanID string) StorageScanView {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		view, err := service.GetStorageScan(scanID)
		if err != nil {
			t.Fatalf("GetStorageScan: %v", err)
		}
		if view.Status != "running" {
			return view
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("storage scan did not finish")
	return StorageScanView{}
}

func TestStorageServiceRequiresAuthorisedRootAndReturnsBoundedEvidence(t *testing.T) {
	root := storageFixture(t)
	service := newTestStorageService(t, root)
	authorised, err := service.SelectStorageRoot()
	if err != nil {
		t.Fatalf("SelectStorageRoot: %v", err)
	}
	if authorised.ID == "" || authorised.DisplayPath != root {
		t.Fatalf("root = %+v", authorised)
	}
	if _, err := service.StartStorageScan("not-authorised", StorageScanOptions{}); err == nil {
		t.Error("unauthorised root accepted")
	}
	scanID, err := service.StartStorageScan(authorised.ID, StorageScanOptions{MaxEntries: 10000})
	if err != nil {
		t.Fatalf("StartStorageScan: %v", err)
	}
	view := waitForStorageScan(t, service, scanID)
	if view.Status != "completed" || view.Partial {
		t.Fatalf("scan view = %+v", view)
	}
	if view.TotalFiles != 4 || view.TotalLogical == 0 {
		t.Errorf("totals = %+v", view)
	}
	if len(view.Duplicates) != 1 || len(view.Duplicates[0].Files) != 2 {
		t.Errorf("duplicates = %+v", view.Duplicates)
	}
	if len(view.Nodes) == 0 || view.Nodes[0].Name == "" {
		t.Errorf("root nodes = %+v", view.Nodes)
	}
	encoded, _ := json.Marshal(view)
	if strings.Contains(string(encoded), root) {
		t.Errorf("absolute root path leaked through storage view: %s", encoded)
	}
	children, err := service.GetStorageNode(scanID, view.Nodes[0].ID)
	if err != nil {
		t.Fatalf("GetStorageNode: %v", err)
	}
	if children.Parent.ID != view.Nodes[0].ID || len(children.Children) == 0 {
		t.Errorf("children = %+v", children)
	}
	if _, err := service.GetStorageNode(scanID, "missing"); err == nil {
		t.Error("unrelated node ID accepted")
	}

	firstPage, err := service.GetStorageNodePage(scanID, view.RootNodeID, "", 2, "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(firstPage.Children) != 2 || firstPage.TotalChildren != 3 || firstPage.NextCursor == "" || !firstPage.Truncated {
		t.Fatalf("first page = %+v", firstPage)
	}
	secondPage, err := service.GetStorageNodePage(scanID, view.RootNodeID, firstPage.NextCursor, 2, "")
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(secondPage.Children) != 1 || secondPage.NextCursor != "" || secondPage.Truncated {
		t.Fatalf("second page = %+v", secondPage)
	}
	searchPage, err := service.GetStorageNodePage(scanID, view.RootNodeID, "", 1, "med")
	if err != nil || len(searchPage.Children) != 1 || searchPage.Children[0].Name != "media" {
		t.Fatalf("full-directory search = %+v, %v", searchPage, err)
	}
	if searchPage.TotalLogical != searchPage.Children[0].LogicalBytes {
		t.Errorf("search total does not describe matched dataset: %+v", searchPage)
	}
	if _, err := service.GetStorageNodePage(scanID, view.RootNodeID, "not-a-cursor", 2, ""); err == nil {
		t.Error("invalid cursor accepted")
	}
}

func TestStorageErrorsSanitizeAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	service := newTestStorageService(t, root)
	authorised, err := service.SelectStorageRoot()
	if err != nil {
		t.Fatal(err)
	}
	service.scanFn = func(ctx context.Context, request storage.Request, progress func(storage.Progress)) (*storage.Result, error) {
		return nil, fmt.Errorf("cannot read C:\\Users\\VeryPrivate\\secret.txt: permission denied")
	}
	scanID, err := service.StartStorageScan(authorised.ID, StorageScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	view := waitForStorageScan(t, service, scanID)
	if view.ErrorCode != "scan_failed" {
		t.Errorf("error code = %q", view.ErrorCode)
	}
	if strings.Contains(view.ErrorMessage, "VeryPrivate") || strings.Contains(view.ErrorMessage, `C:\Users`) {
		t.Errorf("absolute path leaked in error: %q", view.ErrorMessage)
	}
	for _, warning := range view.Warnings {
		if strings.Contains(warning, "VeryPrivate") || strings.Contains(warning, `C:\Users`) {
			t.Errorf("absolute path leaked in warning: %q", warning)
		}
	}
}

func TestStorageSelectErrorSanitizesAbsolutePaths(t *testing.T) {
	service := NewStorageService()
	service.selectDir = func() (string, error) {
		return "", fmt.Errorf("dialog failed for C:\\Users\\VeryPrivate\\secret")
	}
	if _, err := service.SelectStorageRoot(); err == nil || strings.Contains(err.Error(), "VeryPrivate") {
		t.Fatalf("select error leaked a path: %v", err)
	}
}

func TestStorageServiceRejectsSecondRunningScan(t *testing.T) {
	root := storageFixture(t)
	service := newTestStorageService(t, root)
	authorised, err := service.SelectStorageRoot()
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	service.scanFn = func(ctx context.Context, request storage.Request, progress func(storage.Progress)) (*storage.Result, error) {
		progress(storage.Progress{Phase: "enumerating", Items: 1})
		<-release
		return &storage.Result{Root: &storage.Node{ID: "n1", Name: "fixture", Kind: "dir"}, Nodes: map[string]*storage.Node{}}, nil
	}
	first, err := service.StartStorageScan(authorised.ID, StorageScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartStorageScan(authorised.ID, StorageScanOptions{}); err == nil {
		t.Error("second concurrent scan accepted")
	}
	close(release)
	view := waitForStorageScan(t, service, first)
	if view.Status != "completed" {
		t.Errorf("completed scan status = %s", view.Status)
	}
}

func TestStorageServiceCancellationReturnsPartialState(t *testing.T) {
	root := storageFixture(t)
	service := newTestStorageService(t, root)
	authorised, err := service.SelectStorageRoot()
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	service.scanFn = func(ctx context.Context, request storage.Request, progress func(storage.Progress)) (*storage.Result, error) {
		close(started)
		<-ctx.Done()
		return &storage.Result{
			Root:          &storage.Node{ID: "n1", Name: "fixture", Kind: "dir"},
			Nodes:         map[string]*storage.Node{},
			Partial:       true,
			PartialReason: "cancelled",
		}, nil
	}
	scanID, err := service.StartStorageScan(authorised.ID, StorageScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if !service.CancelStorageScan(scanID) {
		t.Fatal("cancel was not accepted")
	}
	view := waitForStorageScan(t, service, scanID)
	if view.Status != "cancelled" || !view.Partial {
		t.Errorf("cancelled view = %+v", view)
	}
}
