//go:build windows && cleanup_execution && m3c2_fixture

package permanent

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/cleanup/execution"
	"github.com/Caseymccallum/StackBroom/internal/cleanup/permanent/harness"
	"github.com/Caseymccallum/StackBroom/internal/storage"
)

func TestM3C2FixtureLockAllowsOnlyVerification(t *testing.T) {
	t.Setenv(harness.EnvHarnessAuthorized, "1")
	t.Setenv(harness.EnvLiveAuthorized, "")
	fixture, err := harness.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	release, err := storage.GuardCleanupAncestors(fixture.Target.File)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	handle, err := openExclusive(fixture.Target.File.Path)
	if err != nil {
		t.Fatal(err)
	}
	item := verifyHandle(handle, fixture.Target)
	if item.Status != ItemValidated {
		t.Fatalf("fixture handle verification = %+v", item)
	}
	if err := syscall.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(fixture.Target.File.Path); err != nil {
		t.Fatalf("verification removed the target: %v", err)
	}
	if err := fixture.VerifySentinels(); err != nil {
		t.Fatal(err)
	}
}

func TestM3C2ContextRejectsTargetOutsideFixture(t *testing.T) {
	t.Setenv(harness.EnvHarnessAuthorized, "1")
	fixture, err := harness.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	outsideRoot := t.TempDir()
	outsidePath := filepath.Join(outsideRoot, "outside.dmp")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(outsidePath, when, when); err != nil {
		t.Fatal(err)
	}
	request := storage.DefaultRequest(outsideRoot)
	request.MinDuplicateSize = ^uint64(0)
	scan, err := storage.NewScanner().Scan(context.Background(), request, nil)
	if err != nil || len(scan.Files) != 1 {
		t.Fatalf("outside scan = %+v, %v", scan, err)
	}
	outside := execution.Target{ID: "outside", File: *scan.Files[0]}
	if _, err := execution.NewContext("outside", RuleID, "1.0.0", fixture.Root,
		[]execution.Target{outside}, time.Now().Add(time.Minute), execution.PermanentOnly); err == nil {
		t.Fatal("context accepted a target outside the fixture root")
	}
}
