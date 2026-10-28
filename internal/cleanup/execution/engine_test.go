package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"github.com/Caseymccallum/StackBroom/internal/storage"
)

type fakeExecutor struct {
	remove  int
	aborted bool
	err     error
	calls   int
}

func (f *fakeExecutor) Execute(_ context.Context, _ *Context, targets []Target, _ func(Progress)) (Result, error) {
	f.calls++
	var items []ItemResult
	for index, target := range targets {
		if index < f.remove {
			_ = os.Remove(target.File.Path)
			items = append(items, ItemResult{TargetID: target.ID, Status: ItemRecycled})
		} else {
			items = append(items, ItemResult{TargetID: target.ID, Status: ItemStillPresent})
		}
	}
	return Result{State: StateVerifying, Items: items, Aborted: f.aborted}, f.err
}

type fakeVerifier struct {
	available bool
	evidence  RecoveryEvidence
	err       error
}

func (f fakeVerifier) Available() error {
	if !f.available {
		return errors.New("unavailable")
	}
	return nil
}

func (f fakeVerifier) Verify(context.Context, Target) (RecoveryEvidence, error) {
	return f.evidence, f.err
}

type fakePreflight struct{ unsupported bool }

func (fakePreflight) Available() error { return nil }
func (f fakePreflight) Check(context.Context, Target) (PreflightEvidence, error) {
	return PreflightEvidence{Supported: !f.unsupported, PolicyKnown: true, CapacityKnown: !f.unsupported,
		Provider: "fake", ObservedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Minute)}, nil
}

type fakeGuard struct{ closed int }

func (fakeGuard) Available() error                                  { return nil }
func (f *fakeGuard) Acquire(context.Context, Target) (Guard, error) { return f, nil }
func (f *fakeGuard) Close() error                                   { f.closed++; return nil }

type fakeReceiptProvider struct{ fail bool }

func (fakeReceiptProvider) Available() error { return nil }
func (f fakeReceiptProvider) Capture(_ context.Context, target Target, _ RecoveryEvidence, now time.Time) (RecycleReceipt, error) {
	if f.fail {
		return RecycleReceipt{}, errors.New("receipt unavailable")
	}
	return RecycleReceipt{TargetID: target.ID, OriginalFileID: target.File.FileID, Provider: "fake",
		OpaqueToken: "opaque", ObservedAt: now, ExpiresAt: now.Add(time.Minute)}, nil
}
func (f fakeReceiptProvider) Rebind(context.Context, RecycleReceipt, Target, time.Time) (RecoveryEvidence, error) {
	return RecoveryEvidence{Available: true, Kind: "fake", Token: "opaque"}, nil
}

type fakeRecorder struct{ records []Record }

func (f *fakeRecorder) Record(record Record) { f.records = append(f.records, record) }

func fixtureExecution(t *testing.T, count int) (*Context, string, func()) {
	t.Helper()
	root := t.TempDir()
	for index := 0; index < count; index++ {
		path := filepath.Join(root, "old-"+string(rune('a'+index))+".dmp")
		if err := os.WriteFile(path, []byte("dump-data"), 0o644); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(-30 * 24 * time.Hour)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	request := storage.DefaultRequest(root)
	request.MinDuplicateSize = ^uint64(0)
	scan, err := storage.NewScanner().Scan(context.Background(), request, nil)
	if err != nil || len(scan.Files) != count {
		t.Fatalf("fixture scan = %+v, %v", scan, err)
	}
	targets := make([]Target, 0, count)
	for index, file := range scan.Files {
		targets = append(targets, Target{ID: "c" + string(rune('1'+index)), File: *file})
	}
	execution, err := NewContext("p1", "crash-dumps-old", "1.0.0", root, targets, time.Now().Add(10*time.Minute), RecycleOnly)
	if err != nil {
		t.Fatal(err)
	}
	return execution, root, func() { _ = os.RemoveAll(root) }
}

func newTestEngine(root string, executor Executor, verifier RecoveryVerifier) *Engine {
	engine := NewEngine(executor, verifier, nil)
	engine.preflight = fakePreflight{}
	engine.finalGuard = &fakeGuard{}
	engine.receipts = fakeReceiptProvider{}
	engine.rootResolver = func(cleanup.Rule) (string, error) { return root, nil }
	return engine
}

func TestEngineCompletesOnlyWithRecoveryEvidence(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 2)
	defer cleanupFixture()
	recorder := &fakeRecorder{}
	engine := newTestEngine(root, &fakeExecutor{remove: 2}, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true, Kind: "recycle_sink", Token: "opaque"}})
	engine.recorder = recorder
	result := engine.Execute(context.Background(), execution, nil)
	if result.State != StateCompleted || len(result.Items) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if len(recorder.records) != 1 || recorder.records[0].TargetCount != 2 {
		t.Errorf("history = %+v", recorder.records)
	}
}

func TestEngineRefusesUnavailableRecoveryBeforeExecution(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 1)
	defer cleanupFixture()
	executor := &fakeExecutor{remove: 1}
	engine := newTestEngine(root, executor, fakeVerifier{available: false})
	result := engine.Execute(context.Background(), execution, nil)
	if result.State != StateFailed || result.ReasonCode != "recovery_unavailable" {
		t.Fatalf("result = %+v", result)
	}
	if executor.calls != 0 {
		t.Error("executor ran without recovery verification")
	}
}

func TestEngineRefusesChangedTargetAndConsumesOnce(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 1)
	defer cleanupFixture()
	path := execution.Targets()[0].File.Path
	if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{remove: 1}
	engine := newTestEngine(root, executor, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true}})
	result := engine.Execute(context.Background(), execution, nil)
	if result.State != StateFailed || result.ReasonCode != "target_changed" || executor.calls != 0 {
		t.Fatalf("result = %+v calls=%d", result, executor.calls)
	}
	if result := engine.Execute(context.Background(), execution, nil); result.ReasonCode != "context" {
		t.Errorf("second execution was not refused: %+v", result)
	}
}

func TestEngineReportsPartialAndAborted(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 2)
	defer cleanupFixture()
	engine := newTestEngine(root, &fakeExecutor{remove: 1, aborted: true}, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true, Kind: "test", Token: "test"}})
	result := engine.Execute(context.Background(), execution, nil)
	if result.State != StatePartial || !result.Aborted {
		t.Fatalf("result = %+v", result)
	}
}

func TestEngineRefusesExpiredContext(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 1)
	defer cleanupFixture()
	engine := newTestEngine(root, &fakeExecutor{remove: 1}, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true}})
	engine.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	result := engine.Execute(context.Background(), execution, nil)
	if result.State != StateFailed || result.ReasonCode != "context" {
		t.Fatalf("expired context result = %+v", result)
	}
}

func TestEngineRefusesChangedRuleVersionAndUnverifiedPrerequisite(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 1)
	defer cleanupFixture()
	engine := newTestEngine(root, &fakeExecutor{remove: 1}, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true}})
	engine.ruleResolver = func(string) (cleanup.Rule, bool) {
		rule, _ := cleanup.RuleByID("crash-dumps-old")
		rule.Version = "2.0.0"
		return rule, true
	}
	if result := engine.Execute(context.Background(), execution, nil); result.ReasonCode != "rule_changed" {
		t.Errorf("rule version result = %+v", result)
	}

	tempRule, _ := cleanup.RuleByID("user-temp-old")
	tempRule.ID = "crash-dumps-old" // Isolate prerequisite validation from rule-ID validation.
	engine.ruleResolver = func(string) (cleanup.Rule, bool) { return tempRule, true }
	second, _, secondCleanup := fixtureExecution(t, 1)
	defer secondCleanup()
	if result := engine.Execute(context.Background(), second, nil); result.ReasonCode != "prerequisite_unverified" {
		t.Errorf("prerequisite result = %+v", result)
	}
}

func TestEngineReportsCancellationAndAccessDenial(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 1)
	defer cleanupFixture()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	engine := newTestEngine(root, &fakeExecutor{remove: 0}, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true}})
	if result := engine.Execute(ctx, execution, nil); result.State != StateCancelled {
		t.Fatalf("cancelled result = %+v", result)
	}

	second, secondRoot, secondCleanup := fixtureExecution(t, 1)
	defer secondCleanup()
	denied := &deniedExecutor{}
	secondEngine := newTestEngine(secondRoot, denied, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true}})
	result := secondEngine.Execute(context.Background(), second, nil)
	if len(result.Items) != 1 || result.Items[0].Status != ItemDenied {
		t.Fatalf("denied result = %+v", result)
	}
}

type deniedExecutor struct{}

func (deniedExecutor) Execute(context.Context, *Context, []Target, func(Progress)) (Result, error) {
	return Result{State: StateVerifying, Items: []ItemResult{{TargetID: "c1", Status: ItemDenied, ReasonCode: "access_denied"}}}, nil
}

func TestEngineAvailabilityDefaultsToFailClosed(t *testing.T) {
	engine := NewEngine(&fakeExecutor{}, fakeVerifier{available: true}, nil)
	if err := engine.Available(); err == nil {
		t.Fatal("engine without native safety providers advertised availability")
	}
}

func TestEngineRefusesUnprovenNoFallbackAndMissingFinalGuard(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 1)
	defer cleanupFixture()
	executor := &fakeExecutor{remove: 1}
	engine := newTestEngine(root, executor, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true}})
	engine.preflight = fakePreflight{unsupported: true}
	if result := engine.Execute(context.Background(), execution, nil); result.ReasonCode != "permanent_fallback_unproven" || executor.calls != 0 {
		t.Fatalf("preflight result = %+v calls=%d", result, executor.calls)
	}

	second, secondRoot, secondCleanup := fixtureExecution(t, 1)
	defer secondCleanup()
	secondExecutor := &fakeExecutor{remove: 1}
	secondEngine := newTestEngine(secondRoot, secondExecutor, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true}})
	secondEngine.finalGuard = NewUnavailableFinalGuard()
	if result := secondEngine.Execute(context.Background(), second, nil); result.ReasonCode != "final_file_guard_unavailable" || secondExecutor.calls != 0 {
		t.Fatalf("guard result = %+v calls=%d", result, secondExecutor.calls)
	}
}

func TestReceiptFailureCannotComplete(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 1)
	defer cleanupFixture()
	engine := newTestEngine(root, &fakeExecutor{remove: 1}, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true, Kind: "test", Token: "token"}})
	engine.receipts = fakeReceiptProvider{fail: true}
	result := engine.Execute(context.Background(), execution, nil)
	if result.State == StateCompleted || result.Items[0].Status != ItemRemovedUnverified {
		t.Fatalf("receipt failure result = %+v", result)
	}
}

func TestFinalGuardClosesOnCompletion(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 1)
	defer cleanupFixture()
	guard := &fakeGuard{}
	engine := newTestEngine(root, &fakeExecutor{remove: 1}, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: true, Kind: "test", Token: "token"}})
	engine.finalGuard = guard
	if result := engine.Execute(context.Background(), execution, nil); result.State != StateCompleted {
		t.Fatalf("result = %+v", result)
	}
	if guard.closed != 1 {
		t.Errorf("guard closed %d times", guard.closed)
	}
}

func TestEngineRefusesUnverifiedRecovery(t *testing.T) {
	execution, root, cleanupFixture := fixtureExecution(t, 1)
	defer cleanupFixture()
	engine := newTestEngine(root, &fakeExecutor{remove: 1}, fakeVerifier{available: true, evidence: RecoveryEvidence{Available: false}})
	result := engine.Execute(context.Background(), execution, nil)
	if result.State != StateFailed || result.Items[0].Status != ItemRemovedUnverified {
		t.Fatalf("result = %+v", result)
	}
}
