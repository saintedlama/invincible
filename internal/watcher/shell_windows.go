//go:build windows

package watcher

import "os/exec"

func shellCommand(cmdStr string) *exec.Cmd {
	return exec.Command("cmd", "/c", cmdStr)
}
