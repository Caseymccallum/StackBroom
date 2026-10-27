// Package run executes external inspection commands with hard time limits.
//
// Rules enforced here, on purpose:
//   - Commands are always executed directly from an argument vector. No shell
//     is involved, so process names, paths or container labels can never be
//     interpreted as shell syntax.
//   - Every call has a deadline. A hung Docker daemon or WSL service can delay a
//     scan, but it cannot hang the application.
//   - Callers receive structured results, including a timed-out or missing
//     binary outcome, instead of an error that has to be string-matched.
package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout bounds a single external command.
const DefaultTimeout = 20 * time.Second

// Result is the outcome of one external command.
type Result struct {
	Command  string
	Args     []string
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
	TimedOut bool
	Missing  bool
	Duration time.Duration
	Timeout  time.Duration
}

// OK reports whether the command ran and exited successfully.
func (r Result) OK() bool {
	return r.Err == nil && r.ExitCode == 0 && !r.TimedOut && !r.Missing
}

// FailureReason renders a short, user-facing explanation. It returns an empty
// string for successful commands.
func (r Result) FailureReason() string {
	switch {
	case r.OK():
		return ""
	case r.Missing:
		return fmt.Sprintf("%s was not found on PATH", r.Command)
	case r.TimedOut:
		return fmt.Sprintf("%s did not respond within %s", r.Command, r.Timeout)
	case r.Err != nil:
		return fmt.Sprintf("%s could not be started: %v", r.Command, r.Err)
	default:
		if line := firstLine(r.Stderr); line != "" {
			return line
		}
		return fmt.Sprintf("%s exited with code %d", r.Command, r.ExitCode)
	}
}

// CommandLine returns the command and arguments for diagnostics. The argument
// vector is fixed by StackBroom, never built from observed system data.
func (r Result) CommandLine() string {
	if len(r.Args) == 0 {
		return r.Command
	}
	return r.Command + " " + strings.Join(r.Args, " ")
}

func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line != "" {
			return line
		}
	}
	return ""
}

// Runner is the seam used by tests to avoid touching real processes, Docker or
// WSL.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) Result
}

// Exec is a Runner that starts real processes.
type Exec struct {
	// Timeout bounds one command. Zero means DefaultTimeout.
	Timeout time.Duration
}

// Run executes the command and waits for it to finish or time out.
func (e Exec) Run(ctx context.Context, name string, args ...string) Result {
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res := Result{Command: name, Args: args, Timeout: timeout}
	started := time.Now()

	cmd := exec.CommandContext(cmdCtx, name, args...)
	// WaitDelay guarantees that inherited pipes are closed shortly after the
	// process is killed, so a timed-out child cannot block the scan.
	cmd.WaitDelay = 2 * time.Second
	configureCommand(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	res.Duration = time.Since(started)
	res.Stdout = stdout.String()
	res.Stderr = stderr.String()

	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}

	if cmdCtx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.Err = cmdCtx.Err()
		return res
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
			return res
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			res.Missing = true
		}
		res.Err = err
	}
	return res
}
