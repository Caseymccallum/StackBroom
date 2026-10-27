//go:build windows

package run

import (
	"os/exec"
	"syscall"
)

// createNoWindow keeps console children (powershell.exe, wsl.exe, docker.exe)
// from flashing a console window when StackBroom is launched from Explorer.
const createNoWindow = 0x08000000

func configureCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
