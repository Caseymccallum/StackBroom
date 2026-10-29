//go:build windows && cleanup_execution && m3c2_fixture

package permanent

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup"
	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
	"github.com/Caseymccallum/StackBroom/internal/cleanup/permanent/harness"
)

// TestM3C2SingleFileLiveDeletion is intentionally skipped unless all three
// external authorisation gates are present. It is not run by ordinary or tagged
// regression tests.
func TestM3C2SingleFileLiveDeletion(t *testing.T) {
	if os.Getenv(harness.EnvHarnessAuthorized) != "1" ||
		os.Getenv(harness.EnvLiveAuthorized) != "1" ||
		os.Getenv("STACKBROOM_ENABLE_PERMANENT_DELETE") != "1" {
		t.Skip("M3C2 live deletion requires explicit harness, live-delete and executor authorisation")
	}
	fixture, err := harness.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	if err := fixture.VerifyInitialState(); err != nil {
		t.Fatal(err)
	}
	if err := fixture.LiveAuthorized(); err != nil {
		t.Fatal(err)
	}
	// The directory-integrity lock is released before the authorised mutation:
	// a parent-directory sharing handle must not be allowed to decide whether
	// the actual disposition can proceed. The fixture remains a uniquely named,
	// owner-created temporary directory whose initial state was just verified.
	if err := fixture.ReleaseLock(); err != nil {
		t.Fatal(err)
	}
	executionContext, err := execution.NewContext("m3c2-live", RuleID, "1.0.0", fixture.Root,
		[]execution.Target{fixture.Target}, time.Now().Add(10*time.Minute), execution.PermanentOnly)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(NewNativeExecutor(), nil)
	engine.rootResolver = func(cleanup.Rule) (string, error) { return fixture.Root, nil }
	result := engine.Execute(context.Background(), executionContext, nil)
	if result.State != StateCompleted || len(result.Items) != 1 || result.Items[0].Status != ItemDeleted {
		t.Fatalf("live result = %+v", result)
	}
	if _, err := os.Lstat(fixture.Target.File.Path); !os.IsNotExist(err) {
		t.Fatalf("target is still present: %v", err)
	}
	if err := fixture.VerifySentinels(); err != nil {
		t.Fatalf("sentinel changed: %v", err)
	}
	summary := result.Summary()
	if summary.RecoveryAvailable || summary.SpaceReclaimedKnown {
		t.Fatalf("fabricated recovery/space claim: %+v", summary)
	}
	if summary.Processed != 1 || summary.Deleted != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}
