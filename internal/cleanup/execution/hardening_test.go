package execution

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestContextCopiesTargetsAndConsumesAtomically(t *testing.T) {
	c, _, cleanup := fixtureExecution(t, 1)
	defer cleanup()
	targets := c.Targets()
	original := targets[0].File.Path
	targets[0].File.Path = "changed"
	if c.Targets()[0].File.Path != original {
		t.Fatal("mutable target escaped")
	}
	var wg sync.WaitGroup
	successes := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); successes <- c.Consume(time.Now()) == nil }()
	}
	wg.Wait()
	close(successes)
	count := 0
	for ok := range successes {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("consumed %d times", count)
	}
}
func TestCancelledEngineNeverCallsExecutor(t *testing.T) {
	c, root, cleanup := fixtureExecution(t, 1)
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	executor := &fakeExecutor{}
	result := newTestEngine(root, executor, fakeVerifier{available: true}).Execute(ctx, c, nil)
	if executor.calls != 0 || result.State != StateCancelled {
		t.Fatalf("%+v calls %d", result, executor.calls)
	}
}
func TestExecutorErrorCannotBecomeCompleted(t *testing.T) {
	c, root, cleanup := fixtureExecution(t, 1)
	defer cleanup()
	result := newTestEngine(root, &fakeExecutor{remove: 1, err: errors.New("failure")}, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true, Kind: "test", Token: "test"}}).Execute(context.Background(), c, nil)
	if result.State == StateCompleted {
		t.Fatal("executor failure hidden")
	}
}
func TestIncompleteEvidenceCannotVerifyRecovery(t *testing.T) {
	c, root, cleanup := fixtureExecution(t, 1)
	defer cleanup()
	result := newTestEngine(root, &fakeExecutor{remove: 1}, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true}}).Execute(context.Background(), c, nil)
	if result.State == StateCompleted || result.Items[0].Status == ItemRecycled {
		t.Fatal("boolean treated as proof")
	}
}

type restoreInspector struct {
	err   error
	calls int
}

func (r *restoreInspector) Available() error { return nil }
func (r *restoreInspector) Inspect(context.Context, RecycleReceipt, Target) error {
	r.calls++
	return r.err
}
func TestRestorePlanningFailsClosed(t *testing.T) {
	c, _, cleanup := fixtureExecution(t, 1)
	defer cleanup()
	target := c.Targets()[0]
	now := time.Now()
	receipt := RecycleReceipt{TargetID: target.ID, OriginalFileID: target.File.FileID, Provider: "fake", OpaqueToken: "fake", ObservedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute)}
	inspector := &restoreInspector{}
	if _, err := PrepareRestore(context.Background(), nil, receipt, target, now); err == nil {
		t.Fatal("missing provider")
	}
	wrong := receipt
	wrong.OriginalFileID = "other"
	if _, err := PrepareRestore(context.Background(), inspector, wrong, target, now); err == nil || inspector.calls != 0 {
		t.Fatal("wrong identity")
	}
	if _, err := PrepareRestore(context.Background(), inspector, receipt, target, now.Add(time.Hour)); err == nil {
		t.Fatal("expired receipt")
	}
	inspector.err = errors.New("collision")
	if _, err := PrepareRestore(context.Background(), inspector, receipt, target, now); err == nil {
		t.Fatal("collision")
	}
	inspector.err = nil
	if _, err := PrepareRestore(context.Background(), inspector, receipt, target, now); err != nil {
		t.Fatal(err)
	}
}
