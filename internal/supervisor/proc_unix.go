//go:build !windows

package supervisor

import (
	"os/exec"
	"syscall"
)

// defaultShell is the "auto" shell on unix.
func defaultShell() Shell { return bashShell{} }

func setProcessGroupAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func termProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) //nolint
	}
}

func KillProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) //nolint
	}
}
