package supervisor

import (
	"os/exec"
	"strings"
)

// Shell is the interpreter used to run a process's cmd and build steps. It
// also knows how to chain multiple build steps into a single command line so
// that shell state (like a `cd`) persists from one step to the next — each
// Shell.Command invocation is one interpreter process, so chaining only
// works within a single invocation, never across separate ones.
type Shell interface {
	Command(cmdStr string) *exec.Cmd
	Join(steps []string) string
}

// bashShell runs commands through bash.
type bashShell struct{}

func (bashShell) Command(cmdStr string) *exec.Cmd {
	return exec.Command("bash", "-c", cmdStr)
}

func (bashShell) Join(steps []string) string {
	return strings.Join(steps, " && ")
}

// pwshShell runs commands through PowerShell 7+ (pwsh). Windows PowerShell
// 5.1 (powershell.exe) is not supported: it predates the `&&`/`||` chain
// operators that build step chaining relies on.
type pwshShell struct{}

func (pwshShell) Command(cmdStr string) *exec.Cmd {
	return exec.Command("pwsh", "-NoProfile", "-Command", cmdStr)
}

func (pwshShell) Join(steps []string) string {
	return strings.Join(steps, " && ")
}

// cmdShell runs commands through Windows cmd.exe.
type cmdShell struct{}

func (cmdShell) Command(cmdStr string) *exec.Cmd {
	return exec.Command("cmd", "/c", cmdStr)
}

// Join concatenates steps with cmd.exe's chain operator without surrounding
// spaces. cmd.exe's argument parsing for builtins like `cd` treats a
// trailing space before `&&` as part of the preceding argument, so
// "cd abc && dir" can resolve "abc " instead of "abc"; "cd abc&&dir" avoids
// the ambiguity entirely.
func (cmdShell) Join(steps []string) string {
	return strings.Join(steps, "&&")
}

// shellFor resolves a project.shell config value to a Shell. "auto" and ""
// resolve to the OS-appropriate default (see defaultShell in proc_windows.go
// / proc_unix.go): bash on Linux/macOS; on Windows, pwsh if found on PATH,
// otherwise cmd.exe.
func shellFor(name string) Shell {
	switch name {
	case "cmd":
		return cmdShell{}
	case "bash":
		return bashShell{}
	case "pwsh":
		return pwshShell{}
	default: // "auto" or ""
		return defaultShell()
	}
}
