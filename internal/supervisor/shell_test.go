package supervisor

import "testing"

func TestBashShell_Join(t *testing.T) {
	got := bashShell{}.Join([]string{"cd abc", "make build"})
	want := "cd abc && make build"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPwshShell_Join(t *testing.T) {
	got := pwshShell{}.Join([]string{"cd abc", "make build"})
	want := "cd abc && make build"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// cmd.exe's argument parsing treats a trailing space before && as part of
// the preceding argument, so steps must be joined without spaces.
func TestCmdShell_Join(t *testing.T) {
	got := cmdShell{}.Join([]string{"cd abc", "make build"})
	want := "cd abc&&make build"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestShellFor(t *testing.T) {
	cases := map[string]Shell{
		"cmd":  cmdShell{},
		"bash": bashShell{},
		"pwsh": pwshShell{},
	}
	for name, want := range cases {
		if got := shellFor(name); got != want {
			t.Errorf("shellFor(%q) = %T, want %T", name, got, want)
		}
	}
	if shellFor("auto") != defaultShell() {
		t.Error("shellFor(\"auto\") should equal defaultShell()")
	}
	if shellFor("") != defaultShell() {
		t.Error("shellFor(\"\") should equal defaultShell()")
	}
}
