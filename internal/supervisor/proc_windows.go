//go:build windows

package supervisor

import (
	"fmt"
	"os/exec"
)

func shellCommand(cmdStr string) *exec.Cmd {
	if sh, err := exec.LookPath("sh"); err == nil {
		return exec.Command(sh, "-c", cmdStr)
	}
	return exec.Command("cmd", "/c", cmdStr)
}

func setProcessGroupAttr(cmd *exec.Cmd) {
}

func termProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	exec.Command("taskkill", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run() //nolint
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run() //nolint
}
