package permanent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
	"github.com/Caseymccallum/StackBroom/internal/storage"
)

type fakeExecutor struct {
	available bool
	items     []ItemResult
	err       error
	calls     int
}

func (f *fakeExecutor) Available() error {
	if f.available {
		return nil
	}
	return ErrDisabled
}
func (f *fakeExecutor) Execute(_ context.Context, targets []execution.Target, _ func(Progress)) (Result, error) {
	f.calls++
	items := append([]ItemResult(nil), f.items...)
	if len(items) == 0 {
		for _, target := range targets {
			items = append(items, ItemResult{TargetID: target.ID, Status: ItemDeleted, Verified: true})
		}
	}
	return Result{State: StateVerifying, Items: items}, f.err
}

type fakeRecorder struct{ records []execution.Record }

func (f *fakeRecorder) Record(record execution.Record) { f.records = append(f.records, record) }

type fixture struct {
	root   string
	rule   cleanup.Rule
	target execution.Target
	now    time.Time
}

func newFixture(t *testing.T, age time.Duration) fixture {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "old.dmp")
	if err := os.WriteFile(path, []byte("crash-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	request := storage.DefaultRequest(root)
	request.MinDuplicateSize = ^uint64(0)
	scan, err := storage.NewScanner().Scan(context.Background(), request, nil)
	if err != nil || len(scan.Files) != 1 {
		t.Fatalf("scan = %+v, %v", scan, err)
	}
	rule := cleanup.Rule{
		ID: RuleID, Version: "1.0.0", MinAgeDays: 7, MaxDepth: 1,
		MaxCandidates: 200, Extensions: []string{"dmp"},
	}
	return fixture{
		root: root, rule: rule, now: time.Now(),
		target: execution.Target{ID: "c1", File: *scan.Files[0]},
	}
}

func newTestEngine(f fixture, executor Executor, recorder execution.Recorder) *Engine {
	engine := NewEngine(executor, recorder)
	engine.now = func() time.Time { return f.now }
	engine.ruleResolver = func(id string) (cleanup.Rule, bool) {
		if id != RuleID {
			return cleanup.Rule{}, false
		}
		return f.rule, true
	}
	engine.rootResolver = func(cleanup.Rule) (string, error) { return f.root, nil }
	return engine
}

func newTestContext(t *testing.T, f fixture, permission execution.Permission) *execution.Context {
	t.Helper()
	executionContext, err := execution.NewContext("e1", RuleID, f.rule.Version, f.root, []execution.Target{f.target}, time.Now().Add(10*time.Minute), permission)
	if err != nil {
		t.Fatal(err)
	}
	return executionContext
}

func TestPermanentEngineCompletesOnlyForVerifiedAbsence(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	executor := &fakeExecutor{available: true}
	recorder := &fakeRecorder{}
	engine := newTestEngine(f, executor, recorder)
	engine.stat = func(string) (bool, error) { return false, nil }
	result := engine.Execute(context.Background(), newTestContext(t, f, execution.PermanentOnly), nil)
	if result.State != StateCompleted || len(result.Items) != 1 || result.Items[0].Status != ItemDeleted {
		t.Fatalf("result = %+v", result)
	}
	summary := result.Summary()
	if summary.Processed != 1 || summary.Deleted != 1 || summary.RecoveryAvailable || summary.SpaceReclaimedKnown {
		t.Fatalf("summary = %+v", summary)
	}
	if len(recorder.records) != 1 || recorder.records[0].DeletionMode != "permanent" || recorder.records[0].RecoveryAvailable {
		t.Fatalf("history = %+v", recorder.records)
	}
}

func TestPermanentEngineCannotReportSuccessWhenFileRemains(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	executor := &fakeExecutor{available: true}
	engine := newTestEngine(f, executor, nil)
	engine.stat = func(string) (bool, error) { return true, nil }
	result := engine.Execute(context.Background(), newTestContext(t, f, execution.PermanentOnly), nil)
	if result.State == StateCompleted || result.Items[0].Status != ItemStillPresent {
		t.Fatalf("false success accepted: %+v", result)
	}
}

func TestPermanentEngineCannotTrustBareExecutorSuccess(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	executor := &fakeExecutor{available: true, items: []ItemResult{{TargetID: "c1", Status: ItemDeleted}}}
	engine := newTestEngine(f, executor, nil)
	engine.stat = func(string) (bool, error) { return false, nil }
	result := engine.Execute(context.Background(), newTestContext(t, f, execution.PermanentOnly), nil)
	if result.State == StateCompleted || result.Items[0].Status != ItemUnknown {
		t.Fatalf("unverified executor success accepted: %+v", result)
	}
}

func TestPermanentEngineRefusesChangedTargetBeforeExecutor(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	if err := os.WriteFile(f.target.File.Path, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A changed target is refused; the executor must not be called.
	executor := &fakeExecutor{available: true}
	result := newTestEngine(f, executor, nil).Execute(context.Background(), newTestContext(t, f, execution.PermanentOnly), nil)
	if result.State != StateFailed || result.ReasonCode != "target_changed" || executor.calls != 0 {
		t.Fatalf("result = %+v calls=%d", result, executor.calls)
	}
}

func TestPermanentEngineRejectsWrongPermissionAndRule(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	executor := &fakeExecutor{available: true}
	engine := newTestEngine(f, executor, nil)
	result := engine.Execute(context.Background(), newTestContext(t, f, execution.RecycleOnly), nil)
	if result.ReasonCode != "permission" || executor.calls != 0 {
		t.Fatalf("permission result = %+v calls=%d", result, executor.calls)
	}
	engine.ruleResolver = func(string) (cleanup.Rule, bool) {
		wrong := f.rule
		wrong.ID = "user-temp-old"
		return wrong, true
	}
	result = engine.Execute(context.Background(), newTestContext(t, f, execution.PermanentOnly), nil)
	if result.ReasonCode != "rule_changed" {
		t.Fatalf("rule result = %+v", result)
	}
}

func TestPermanentEngineConsumesContextOnce(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	executor := &fakeExecutor{available: true}
	engine := newTestEngine(f, executor, nil)
	engine.stat = func(string) (bool, error) { return false, nil }
	executionContext := newTestContext(t, f, execution.PermanentOnly)
	if result := engine.Execute(context.Background(), executionContext, nil); result.State != StateCompleted {
		t.Fatalf("first result = %+v", result)
	}
	if result := engine.Execute(context.Background(), executionContext, nil); result.ReasonCode != "context" || executor.calls != 1 {
		t.Fatalf("second result = %+v calls=%d", result, executor.calls)
	}
}

func TestPermanentEnginePartialAndCancelled(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	secondPath := filepath.Join(f.root, "second.dmp")
	if err := os.WriteFile(secondPath, []byte("second-crash-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(secondPath, when, when); err != nil {
		t.Fatal(err)
	}
	request := storage.DefaultRequest(f.root)
	request.MinDuplicateSize = ^uint64(0)
	scan, err := storage.NewScanner().Scan(context.Background(), request, nil)
	if err != nil {
		t.Fatal(err)
	}
	var second execution.Target
	for _, file := range scan.Files {
		if file.Name == "second.dmp" {
			second = execution.Target{ID: "c2", File: *file}
			break
		}
	}
	if second.ID == "" {
		t.Fatal("second dump not found")
	}
	executionContext, err := execution.NewContext("e2", RuleID, f.rule.Version, f.root,
		[]execution.Target{f.target, second}, time.Now().Add(10*time.Minute), execution.PermanentOnly)
	if err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{available: true, items: []ItemResult{
		{TargetID: "c1", Status: ItemDeleted, Verified: true}, {TargetID: "c2", Status: ItemRefused, ReasonCode: "in_use"},
	}}
	engine := newTestEngine(f, executor, nil)
	engine.stat = func(string) (bool, error) { return false, nil }
	result := engine.Execute(context.Background(), executionContext, nil)
	if result.State != StatePartial || result.Summary().Processed != 2 || result.Summary().Deleted != 1 || result.Summary().Refused != 1 {
		t.Fatalf("partial result = %+v summary=%+v", result, result.Summary())
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	executor.calls = 0
	result = engine.Execute(cancelled, newTestContext(t, f, execution.PermanentOnly), nil)
	if result.State != StateCancelled || executor.calls != 0 {
		t.Fatalf("cancel result = %+v calls=%d", result, executor.calls)
	}
}

func TestPermanentEngineRefusesUnavailableExecutor(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	result := newTestEngine(f, &fakeExecutor{}, nil).Execute(context.Background(), newTestContext(t, f, execution.PermanentOnly), nil)
	if result.ReasonCode != "execution_unavailable" {
		t.Fatalf("result = %+v", result)
	}
}

func TestPermanentEngineRefusesExpiredContext(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	executionContext, err := execution.NewContext("e3", RuleID, f.rule.Version, f.root,
		[]execution.Target{f.target}, time.Now().Add(time.Minute), execution.PermanentOnly)
	if err != nil {
		t.Fatal(err)
	}
	engine := newTestEngine(f, &fakeExecutor{available: true}, nil)
	engine.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	result := engine.Execute(context.Background(), executionContext, nil)
	if result.ReasonCode != "context" {
		t.Fatalf("result = %+v", result)
	}
}

func TestPermanentEngineRefusesScopeEscape(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	engine := newTestEngine(f, &fakeExecutor{available: true}, nil)
	engine.rootResolver = func(cleanup.Rule) (string, error) { return t.TempDir(), nil }
	result := engine.Execute(context.Background(), newTestContext(t, f, execution.PermanentOnly), nil)
	if result.State != StateFailed || result.ReasonCode != "scope_changed" {
		t.Fatalf("result = %+v", result)
	}
}

func TestPermanentEngineRefusesNonDumpTarget(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	f.target.File.Name = "old.txt"
	executor := &fakeExecutor{available: true}
	result := newTestEngine(f, executor, nil).Execute(context.Background(), newTestContext(t, f, execution.PermanentOnly), nil)
	if result.State != StateFailed || result.ReasonCode != "target_changed" || executor.calls != 0 {
		t.Fatalf("non-dump result = %+v calls=%d", result, executor.calls)
	}
}

func TestPermanentEngineRefusesHardLink(t *testing.T) {
	f := newFixture(t, 30*24*time.Hour)
	linkPath := filepath.Join(f.root, "linked.dmp")
	if err := os.Link(f.target.File.Path, linkPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	executor := &fakeExecutor{available: true}
	result := newTestEngine(f, executor, nil).Execute(context.Background(), newTestContext(t, f, execution.PermanentOnly), nil)
	if result.State != StateFailed || result.ReasonCode != "target_changed" || executor.calls != 0 {
		t.Fatalf("hard-link result = %+v calls=%d", result, executor.calls)
	}
}

func TestPermanentSummaryNeverClaimsRecovery(t *testing.T) {
	result := Result{Items: []ItemResult{
		{Status: ItemDeleted, AllocatedKnown: true, AllocatedBytes: 100, SpaceReclaimed: 100},
		{Status: ItemStillPresent}, {Status: ItemRefused}, {Status: ItemChanged}, {Status: ItemUnknown},
	}}
	summary := result.Summary()
	if summary.Requested != 5 || summary.Processed != 5 || summary.Deleted != 1 || summary.Refused != 1 || summary.Changed != 1 || summary.StillPresent != 1 || summary.Unknown != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.RecoveryAvailable || summary.SpaceReclaimedKnown || summary.SpaceReclaimedBytes != 100 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestPermanentStubRefusesInOrdinaryBuild(t *testing.T) {
	executor := NewNativeExecutor()
	if err := executor.Available(); err == nil {
		t.Skip("a tagged, explicitly enabled permanent executor is present")
	}
	result, err := executor.Execute(context.Background(), nil, nil)
	if err == nil || result.State != StateFailed {
		t.Fatalf("result = %+v err=%v", result, err)
	}
}
