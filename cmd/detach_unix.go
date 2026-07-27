//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

// detachProcess puts the child in its own process group so that Ctrl+C in the
// parent's terminal, or closing that terminal, does not take it down with it.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
