//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setDetachAttr sets the SysProcAttr to detach the daemon process from the terminal.
func setDetachAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}