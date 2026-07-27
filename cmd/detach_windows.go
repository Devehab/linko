//go:build windows

package cmd

import (
	"os/exec"
	"syscall"
)

// detachProcess starts the child without a console of its own, so it keeps
// running after the launching window closes.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008 | 0x08000000, // DETACHED_PROCESS | CREATE_NO_WINDOW
	}
}
