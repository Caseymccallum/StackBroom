package cli

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestGoldenScanOutput(t *testing.T) {
	app, out, _ := newTestApp(fixtureSnapshot())
	app.HomeDir = `C:\Users\Casey`
	if code := app.Run(context.Background(), []string{"scan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	checkGolden(t, "scan", out.String())
}

func TestGoldenPlanOutput(t *testing.T) {
	app, out, _ := newTestApp(fixtureSnapshot())
	app.HomeDir = `C:\Users\Casey`
	if code := app.Run(context.Background(), []string{"plan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	checkGolden(t, "plan", out.String())
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	got = strings.ReplaceAll(got, "\r\n", "\n")
	path := filepath.Join("testdata", name+".golden.txt")

	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("create testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file (run `go test ./internal/cli -run TestGolden -update`): %v", err)
	}
	want := strings.ReplaceAll(string(wantBytes), "\r\n", "\n")
	if got != want {
		t.Errorf("output changed; re-run with -update if the change is intended\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
