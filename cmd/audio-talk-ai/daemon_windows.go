//go:build windows

package main

import "os/exec"

// setDetachAttr is a no-op on Windows; process detachment is handled differently.
func setDetachAttr(cmd *exec.Cmd) {
	// No-op: Windows handles background processes differently.
}