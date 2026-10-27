package run

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFailureReason(t *testing.T) {
	tests := []struct {
		name string
		in   Result
		want string
	}{
		{"success", Result{Command: "docker", ExitCode: 0}, ""},
		{"missing", Result{Command: "docker", Missing: true}, "docker was not found on PATH"},
		{
			"timeout",
			Result{Command: "docker", TimedOut: true, Timeout: 5 * time.Second},
			"docker did not respond within 5s",
		},
		{
			"stderr first line",
			Result{Command: "docker", ExitCode: 1, Stderr: "\nCannot connect to the Docker daemon\nmore"},
			"Cannot connect to the Docker daemon",
		},
		{
			"exit code fallback",
			Result{Command: "docker", ExitCode: 3},
			"docker exited with code 3",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.FailureReason(); got != tc.want {
				t.Fatalf("FailureReason() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExecRunsAndCapturesOutput(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("StackBroom's runner tests use Windows commands")
	}
	res := Exec{Timeout: 10 * time.Second}.Run(context.Background(), "cmd.exe", "/c", "echo hello")
	if !res.OK() {
		t.Fatalf("command failed: %s", res.FailureReason())
	}
	if !strings.Contains(res.Stdout, "hello") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
}

func TestExecMissingBinary(t *testing.T) {
	res := Exec{Timeout: time.Second}.Run(context.Background(), "stackbroom-definitely-not-a-real-binary")
	if !res.Missing {
		t.Fatalf("expected a missing-binary result, got %+v", res)
	}
	if !strings.Contains(res.FailureReason(), "not found") {
		t.Fatalf("reason = %q", res.FailureReason())
	}
}

func TestExecEnforcesTimeout(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("timeout test uses PowerShell")
	}
	start := time.Now()
	res := Exec{Timeout: 500 * time.Millisecond}.Run(context.Background(),
		"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 10")
	elapsed := time.Since(start)

	if !res.TimedOut {
		t.Fatalf("expected a timeout, got %+v", res)
	}
	if elapsed > 8*time.Second {
		t.Fatalf("timeout was not enforced promptly: %s", elapsed)
	}
	if res.Timeout != 500*time.Millisecond {
		t.Errorf("reported timeout = %s", res.Timeout)
	}
}

func TestExecTreatsTheNameAsOneExecutable(t *testing.T) {
	// If a shell were involved, "cmd.exe /c echo hi" would be executed. A
	// direct argv execution instead looks for an executable with that literal
	// name and fails.
	res := Exec{Timeout: time.Second}.Run(context.Background(), "cmd.exe /c echo hi")
	if !res.Missing {
		t.Fatalf("command string was interpreted instead of treated literally: %+v", res)
	}
}
