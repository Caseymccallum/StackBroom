//go:build !windows

package run

import "os/exec"

func configureCommand(cmd *exec.Cmd) {
	// StackBroom is Windows-first. On other platforms the collectors report
	// themselves unavailable; the runner still works for tests.
	_ = cmd
}
