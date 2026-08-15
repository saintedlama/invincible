//go:build windows

package supervisor

import (
	"errors"
	"testing"
)

func TestDefaultShellFor_PwshFound(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "pwsh" {
			return `C:\Program Files\PowerShell\7\pwsh.exe`, nil
		}
		return "", errors.New("not found")
	}
	if got := defaultShellFor(lookPath); got != (pwshShell{}) {
		t.Errorf("got %T, want pwshShell", got)
	}
}

func TestDefaultShellFor_PwshMissing(t *testing.T) {
	lookPath := func(name string) (string, error) {
		return "", errors.New("not found")
	}
	if got := defaultShellFor(lookPath); got != (cmdShell{}) {
		t.Errorf("got %T, want cmdShell", got)
	}
}
