package main

import (
	"context"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
)

func TestCleanupPreviewViewCannotMutateBackend(t *testing.T) {
	service := NewCleanupService()
	view := CleanupPreviewView{Selected: []CleanupCandidate{{ID: "original"}}, Risks: []string{"risk"}, SideEffects: []string{"effect"}, Excluded: []CleanupExclusion{{Reason: "excluded"}}}
	service.previews["p"] = &cleanupPreviewState{View: cloneCleanupPreview(view), ExpiresAt: time.Now().Add(time.Hour)}
	view.Selected[0].ID = "changed"
	got, err := service.GetCleanupPreview("p")
	if err != nil {
		t.Fatal(err)
	}
	got.Selected[0].ID = "changed"
	got.Risks[0] = "changed"
	got.SideEffects[0] = "changed"
	got.Excluded[0].Reason = "changed"
	next, _ := service.GetCleanupPreview("p")
	if next.Selected[0].ID != "original" || next.Risks[0] != "risk" || next.Excluded[0].Reason != "excluded" || next.SideEffects[0] != "effect" {
		t.Fatal("mutable preview")
	}
}
func TestCleanupExecutionCancellationAndShutdown(t *testing.T) {
	service := NewCleanupService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.running["p"] = cancel
	service.executionProgress["p"] = execution.Progress{State: execution.StateExecuting, Completed: 1, Total: 2}
	if !service.CancelCleanupExecution("p") || ctx.Err() != context.Canceled {
		t.Fatal("cancel failed")
	}
	if service.CancelCleanupExecution("unknown") {
		t.Fatal("unknown execution")
	}
	p, err := service.GetCleanupExecutionProgress("p")
	if err != nil || p.Completed != 1 {
		t.Fatal("progress")
	}
	second, stop := context.WithCancel(context.Background())
	defer stop()
	service.running["q"] = stop
	if service.ServiceShutdown() != nil || second.Err() != context.Canceled || !service.closed {
		t.Fatal("shutdown")
	}
}
