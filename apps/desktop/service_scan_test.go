package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func TestCancelAndOverlap(t *testing.T) {
	started := make(chan struct{})
	service := testService()
	service.scanFn = func(ctx context.Context, _ string) model.Snapshot {
		close(started)
		<-ctx.Done()
		return testSnapshot()
	}

	done := make(chan QuickResult, 1)
	go func() {
		result, err := service.QuickScan()
		if err != nil {
			t.Errorf("QuickScan: %v", err)
		}
		done <- result
	}()
	<-started

	if _, err := service.FullScan(); !errors.Is(err, ErrScanInProgress) {
		t.Fatalf("overlap error = %v, want ErrScanInProgress", err)
	}
	if !service.CancelScan() {
		t.Fatal("CancelScan returned false while a scan was active")
	}
	result := <-done
	if result.Status != "cancelled" {
		t.Errorf("status = %q, want cancelled", result.Status)
	}
	if service.CancelScan() {
		t.Error("CancelScan returned true with no active scan")
	}
}

func TestLargeProcessInventory(t *testing.T) {
	service := testService()
	service.scanFn = func(context.Context, string) model.Snapshot {
		snapshot := testSnapshot()
		snapshot.Processes = make([]model.Process, 5000)
		for i := range snapshot.Processes {
			snapshot.Processes[i] = model.Process{
				PID: int32(i + 1), Name: "proc.exe", Category: model.CategoryOther,
				Runtime: model.RuntimeUnknown, MemoryKnown: i%2 == 0, WorkingSetBytes: uint64(i) * 1024,
			}
		}
		return snapshot
	}
	result, err := service.FullScan()
	if err != nil {
		t.Fatalf("FullScan: %v", err)
	}
	if len(result.Processes) != 5000 {
		t.Errorf("processes = %d", len(result.Processes))
	}
}

func TestScanStatus(t *testing.T) {
	if got := scanStatus(context.Background()); got != "ok" {
		t.Errorf("background = %q", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := scanStatus(ctx); got != "cancelled" {
		t.Errorf("cancelled = %q", got)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if got := scanStatus(ctx); got != "timeout" {
		t.Errorf("deadline = %q", got)
	}
}
