package wsl

import (
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/Caseymccallum/StackBroom/internal/run"
)

type fakeRunner struct {
	result run.Result
	calls  []string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) run.Result {
	_ = ctx
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	r := f.result
	r.Command = name
	r.Args = args
	return r
}

func utf16leString(s string) string {
	units := utf16.Encode([]rune(s))
	buf := make([]byte, 0, len(units)*2)
	for _, u := range units {
		var pair [2]byte
		binary.LittleEndian.PutUint16(pair[:], u)
		buf = append(buf, pair[:]...)
	}
	return string(buf)
}

func TestCollectDecodesUTF16WithoutBOM(t *testing.T) {
	// This is the exact shape wsl.exe produced on the development machine:
	// UTF-16LE, no BOM.
	out := utf16leString("  NAME              STATE           VERSION\r\n" +
		"* docker-desktop    Running         2\r\n")

	runner := &fakeRunner{result: run.Result{Stdout: out}}
	info := Inspector{Runner: runner}.Collect(context.Background())

	if !info.Available {
		t.Fatalf("wsl should be available: %+v", info)
	}
	if len(info.Distributions) != 1 {
		t.Fatalf("distributions = %+v", info.Distributions)
	}
	d := info.Distributions[0]
	if d.Name != "docker-desktop" || d.State != "Running" || d.Version != "2" || !d.IsDefault {
		t.Errorf("distribution = %+v", d)
	}
	if info.RunningDistributions() != 1 {
		t.Errorf("running count = %d", info.RunningDistributions())
	}
}

func TestCollectNoDistributions(t *testing.T) {
	runner := &fakeRunner{result: run.Result{
		ExitCode: 1,
		Stderr:   "Windows Subsystem for Linux has no installed distributions.",
	}}
	info := Inspector{Runner: runner}.Collect(context.Background())

	if !info.Available {
		t.Fatal("WSL should be reported available when it exists but has no distributions")
	}
	if len(info.Distributions) != 0 {
		t.Errorf("distributions = %+v", info.Distributions)
	}
	if !strings.Contains(strings.ToLower(info.Reason), "no distributions") {
		t.Errorf("reason = %q", info.Reason)
	}
}

func TestCollectUnavailableReasons(t *testing.T) {
	tests := []struct {
		name   string
		result run.Result
		want   string
	}{
		{"missing binary", run.Result{Missing: true}, "not found"},
		{"timeout", run.Result{TimedOut: true, Timeout: 5 * time.Second}, "did not respond"},
		{
			name:   "feature not enabled",
			result: run.Result{ExitCode: 1, Stderr: "Windows Subsystem for Linux is not installed. You can install it by running: wsl.exe --install"},
			want:   "not installed",
		},
		{
			name:   "needs update",
			result: run.Result{ExitCode: 1, Stderr: "WSL 2 requires an update to its kernel component."},
			want:   "update",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{result: tc.result}
			info := Inspector{Runner: runner}.Collect(context.Background())
			if info.Available {
				t.Fatalf("expected unavailable, got %+v", info)
			}
			if !strings.Contains(strings.ToLower(info.Reason), tc.want) {
				t.Errorf("reason = %q, want it to contain %q", info.Reason, tc.want)
			}
		})
	}
}

func TestCollectIssuesOnlyListCommand(t *testing.T) {
	runner := &fakeRunner{result: run.Result{Stdout: "  NAME  STATE  VERSION\n"}}
	Inspector{Runner: runner}.Collect(context.Background())

	if len(runner.calls) != 1 {
		t.Fatalf("expected one call, got %v", runner.calls)
	}
	if !strings.Contains(runner.calls[0], "--list --verbose") {
		t.Fatalf("unexpected WSL command: %s", runner.calls[0])
	}
	for _, forbidden := range []string{"--terminate", "--shutdown", "-t ", "--exec", "-e ", "--run"} {
		if strings.Contains(runner.calls[0], forbidden) {
			t.Fatalf("StackBroom issued a state-changing WSL command: %s", runner.calls[0])
		}
	}
}
