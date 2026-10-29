//go:build windows && cleanup_execution && m3c2_fixture

package permanent

import (
	"bytes"
	"context"
	"os"
	"os/exec"
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
	item := verifyHandle(handle, fixture.Root, fixture.Target)
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

func TestPermanentWorkerRejectsCallerSuppliedOutsideRoot(t *testing.T) {
	t.Setenv(harness.EnvHarnessAuthorized, "1")
	fixture, err := harness.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	item := deleteOne(context.Background(), filepath.Join(t.TempDir(), "not-trusted"), fixture.Target)
	if item.Status != ItemRefused || item.ReasonCode != "scope" {
		t.Fatalf("outside trusted root was not refused: %+v", item)
	}
	if _, err := os.Lstat(fixture.Target.File.Path); err != nil {
		t.Fatalf("outside-root refusal touched the fixture: %v", err)
	}
}

func prepareRefusalFixture(t *testing.T) *harness.Fixture {
	t.Helper()
	t.Setenv(harness.EnvHarnessAuthorized, "1")
	fixture, err := harness.Prepare()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fixture.Close() })
	return fixture
}

func assertOriginalTargetAndSentinelsIntact(t *testing.T, fixture *harness.Fixture) {
	t.Helper()
	body, err := os.ReadFile(fixture.Target.File.Path)
	if err != nil {
		t.Fatalf("protected target was removed: %v", err)
	}
	if !bytes.Equal(body, fixture.Contents) {
		t.Fatal("protected target contents changed")
	}
	assertSentinelsIntact(t, fixture)
}

func assertSentinelsIntact(t *testing.T, fixture *harness.Fixture) {
	t.Helper()
	if err := fixture.VerifySentinels(); err != nil {
		t.Fatalf("unselected sentinel changed: %v", err)
	}
}

func setFixtureAttributes(t *testing.T, path string, attributes uint32) {
	t.Helper()
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(pointer, attributes); err != nil {
		t.Skipf("could not set fixture attributes: %v", err)
	}
}

func TestM3C2FixtureExecutorRequiresHarnessAuthorization(t *testing.T) {
	t.Setenv(harness.EnvHarnessAuthorized, "")
	t.Setenv("STACKBROOM_ENABLE_PERMANENT_DELETE", "1")
	executor := NewFixtureExecutor(t.TempDir())
	if err := executor.Available(); err == nil {
		t.Fatal("fixture executor was available without harness authorisation")
	}
}

func TestM3C2FixtureExecutorRejectsArbitraryTemporaryRoot(t *testing.T) {
	t.Setenv(harness.EnvHarnessAuthorized, "1")
	t.Setenv("STACKBROOM_ENABLE_PERMANENT_DELETE", "1")
	executor := NewFixtureExecutor(t.TempDir())
	if err := executor.Available(); err == nil {
		t.Fatal("fixture executor accepted an arbitrary temporary root")
	}
}

func TestM3C2HiddenTargetRefusedWithoutMutation(t *testing.T) {
	fixture := prepareRefusalFixture(t)
	setFixtureAttributes(t, fixture.Target.File.Path, fileAttributeHidden)
	item := deleteOne(context.Background(), fixture.Root, fixture.Target)
	if item.Status != ItemRefused || item.ReasonCode != "hidden" {
		t.Fatalf("hidden target was not refused: %+v", item)
	}
	assertOriginalTargetAndSentinelsIntact(t, fixture)
}

func TestM3C2SystemTargetRefusedWithoutMutation(t *testing.T) {
	fixture := prepareRefusalFixture(t)
	setFixtureAttributes(t, fixture.Target.File.Path, fileAttributeSystem)
	item := deleteOne(context.Background(), fixture.Root, fixture.Target)
	if item.Status != ItemRefused || item.ReasonCode != "system" {
		t.Fatalf("system target was not refused: %+v", item)
	}
	assertOriginalTargetAndSentinelsIntact(t, fixture)
}

func TestM3C2LockedTargetRefusedWithoutMutation(t *testing.T) {
	fixture := prepareRefusalFixture(t)
	pointer, err := syscall.UTF16PtrFromString(fixture.Target.File.Path)
	if err != nil {
		t.Fatal(err)
	}
	holder, err := syscall.CreateFile(pointer, syscall.GENERIC_READ, 0, nil, 3, fileFlagOpenReparsePoint, 0)
	if err != nil {
		t.Skipf("could not hold the fixture target open: %v", err)
	}
	item := deleteOne(context.Background(), fixture.Root, fixture.Target)
	if closeErr := syscall.CloseHandle(holder); closeErr != nil {
		t.Fatalf("close lock holder: %v", closeErr)
	}
	if item.Status != ItemRefused || item.ReasonCode != "in_use" {
		t.Fatalf("locked target was not refused: %+v", item)
	}
	assertOriginalTargetAndSentinelsIntact(t, fixture)
}

func TestM3C2HardLinkedTargetRefusedWithoutMutation(t *testing.T) {
	fixture := prepareRefusalFixture(t)
	if err := fixture.ReleaseLock(); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(fixture.Root, "second-link.dmp")
	if err := os.Link(fixture.Target.File.Path, linkPath); err != nil {
		t.Skipf("could not create the fixture hard link: %v", err)
	}
	defer os.Remove(linkPath)
	item := deleteOne(context.Background(), fixture.Root, fixture.Target)
	if item.Status != ItemRefused || item.ReasonCode != "hard_link" {
		t.Fatalf("hard-linked target was not refused: %+v", item)
	}
	assertOriginalTargetAndSentinelsIntact(t, fixture)
}

func TestM3C2ModifiedTargetRefusedWithoutMutation(t *testing.T) {
	fixture := prepareRefusalFixture(t)
	if err := os.WriteFile(fixture.Target.File.Path, []byte("changed-after-scan"), 0o600); err != nil {
		t.Fatal(err)
	}
	item := deleteOne(context.Background(), fixture.Root, fixture.Target)
	if item.Status != ItemChanged || item.ReasonCode != "metadata_changed" {
		t.Fatalf("modified target was not refused: %+v", item)
	}
	body, err := os.ReadFile(fixture.Target.File.Path)
	if err != nil || string(body) != "changed-after-scan" {
		t.Fatalf("modified target was not left untouched: %q, %v", body, err)
	}
	assertSentinelsIntact(t, fixture)
}

func TestM3C2RenamedTargetRefusedWithoutMutation(t *testing.T) {
	fixture := prepareRefusalFixture(t)
	if err := fixture.ReleaseLock(); err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(fixture.Root, "renamed.dmp")
	if err := os.Rename(fixture.Target.File.Path, renamed); err != nil {
		t.Fatal(err)
	}
	item := deleteOne(context.Background(), fixture.Root, fixture.Target)
	if item.Status != ItemChanged || item.ReasonCode != "inaccessible" {
		t.Fatalf("renamed target was not refused: %+v", item)
	}
	body, err := os.ReadFile(renamed)
	if err != nil || !bytes.Equal(body, fixture.Contents) {
		t.Fatalf("renamed target was removed or changed: %v", err)
	}
	assertSentinelsIntact(t, fixture)
}

func TestM3C2ReparsePointRefusedWithoutFollowing(t *testing.T) {
	fixture := prepareRefusalFixture(t)
	if err := fixture.ReleaseLock(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(fixture.Target.File.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("keep-a.txt", fixture.Target.File.Path); err != nil {
		t.Skipf("could not create a fixture reparse point: %v", err)
	}
	item := deleteOne(context.Background(), fixture.Root, fixture.Target)
	if item.Status != ItemRefused || item.ReasonCode != "not_regular" {
		t.Fatalf("reparse point was not refused: %+v", item)
	}
	info, err := os.Lstat(fixture.Target.File.Path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("reparse point was removed or followed: %v, %v", info, err)
	}
	assertSentinelsIntact(t, fixture)
}

func TestM3C2JunctionTargetRefusedWithoutFollowing(t *testing.T) {
	fixture := prepareRefusalFixture(t)
	if err := fixture.ReleaseLock(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(fixture.Target.File.Path); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	command := exec.Command("cmd", "/c", "mklink", "/J", fixture.Target.File.Path, outside)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("could not create a target junction: %v (%s)", err, output)
	}
	defer os.Remove(fixture.Target.File.Path)
	item := deleteOne(context.Background(), fixture.Root, fixture.Target)
	if item.Status != ItemRefused || item.ReasonCode != "not_regular" {
		t.Fatalf("junction target was not refused: %+v", item)
	}
	if _, err := os.Lstat(fixture.Target.File.Path); err != nil {
		t.Fatalf("junction target was removed or followed: %v", err)
	}
	assertSentinelsIntact(t, fixture)
}

func TestM3C2ReparseAncestorRefusedWithoutMutation(t *testing.T) {
	fixture := prepareRefusalFixture(t)
	if err := fixture.ReleaseLock(); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideTarget := filepath.Join(outside, "probe.dmp")
	if err := os.WriteFile(outsideTarget, fixture.Contents, 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(outsideTarget, when, when); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(fixture.Root, "reparse-dir")
	command := exec.Command("cmd", "/c", "mklink", "/J", linkPath, outside)
	if output, err := command.CombinedOutput(); err != nil {
		t.Skipf("could not create a fixture junction: %v (%s)", err, output)
	}
	defer os.Remove(linkPath)
	target := fixture.Target
	target.File.Path = filepath.Join(linkPath, "probe.dmp")
	target.File.Name = "probe.dmp"
	item := deleteOne(context.Background(), fixture.Root, target)
	if item.Status != ItemRefused || item.ReasonCode != "scope" {
		t.Fatalf("reparse ancestor was not refused: %+v", item)
	}
	if _, err := os.Lstat(outsideTarget); err != nil {
		t.Fatalf("reparse target was removed: %v", err)
	}
	if _, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("reparse junction was removed: %v", err)
	}
	assertSentinelsIntact(t, fixture)
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
