//go:build !windows

package watcher

import "os/exec"

func shellCommand(cmdStr string) *exec.Cmd {
	return exec.Command("sh", "-c", cmdStr)
}
