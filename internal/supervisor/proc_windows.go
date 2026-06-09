//go:build windows

package supervisor

import (
	"fmt"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func shellCommand(cmdStr string) *exec.Cmd {
	if sh, err := exec.LookPath("sh"); err == nil {
		return exec.Command(sh, "-c", cmdStr)
	}
	return exec.Command("cmd", "/c", cmdStr)
}

func setProcessGroupAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func termProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	// Send Ctrl+Break to the process group — Go's os/signal handles this
	// as os.Interrupt, unlike CTRL_CLOSE_EVENT from taskkill which Go ignores.
	windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid)) //nolint
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid)).Run() //nolint
}
