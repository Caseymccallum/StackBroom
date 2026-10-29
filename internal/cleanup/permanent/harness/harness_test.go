//go:build windows && m3c2_fixture

package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixtureHarnessRequiresAuthorizationAndDoesNotDelete(t *testing.T) {
	t.Setenv(EnvHarnessAuthorized, "0")
	t.Setenv(EnvLiveAuthorized, "")
	if _, err := Prepare(); err != ErrNotAuthorized {
		t.Fatalf("unauthorised prepare = %v", err)
	}

	t.Setenv(EnvHarnessAuthorized, "1")
	f, err := Prepare()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.VerifyInitialState(); err != nil {
		t.Fatal(err)
	}
	if err := f.LiveAuthorized(); err != ErrLiveNotAuthorized {
		t.Fatalf("live authorisation = %v", err)
	}
	if _, err := os.Lstat(f.Target.File.Path); err != nil {
		t.Fatalf("fixture target is absent: %v", err)
	}
	if len(f.Target.File.FileID) == 0 {
		t.Fatal("fixture target has no Windows identity")
	}
	if f.Root == "" || f.Target.ID == "" {
		t.Fatal("fixture root or target ID is missing")
	}
	if !fixtureRootValid(f.Root) {
		t.Fatal("generated fixture root is not isolated")
	}
	if fixtureRootValid(t.TempDir()) {
		t.Fatal("an arbitrary temporary directory was accepted as a fixture root")
	}
	sensitive := filepath.Join(t.TempDir(), "CrashDumps", "StackBroom-M3C2-nope")
	if err := os.MkdirAll(sensitive, 0o700); err != nil {
		t.Fatal(err)
	}
	if fixtureRootValid(sensitive) {
		t.Fatal("a fixture path under a sensitive location was accepted")
	}
}
