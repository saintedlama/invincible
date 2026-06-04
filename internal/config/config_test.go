package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeToml(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".invincible.toml")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_Valid(t *testing.T) {
	path := writeToml(t, `
[project]
name = "testapp"

[[process]]
name = "api"
cmd = "echo hello"
port = 8080
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Name != "testapp" {
		t.Errorf("project name: got %q, want testapp", cfg.Project.Name)
	}
	if len(cfg.Processes) != 1 {
		t.Fatalf("processes: got %d, want 1", len(cfg.Processes))
	}
	p := cfg.Processes[0]
	if p.Name != "api" {
		t.Errorf("name: got %q", p.Name)
	}
	if p.Cmd != "echo hello" {
		t.Errorf("cmd: got %q", p.Cmd)
	}
	if p.Port != 8080 {
		t.Errorf("port: got %d", p.Port)
	}
	if p.PortEnv != "PORT" {
		t.Errorf("port_env default: got %q", p.PortEnv)
	}
}

func TestLoad_MultipleProcesses(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "api"
cmd = "echo api"

[[process]]
name = "worker"
cmd = "echo worker"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Processes) != 2 {
		t.Errorf("got %d processes, want 2", len(cfg.Processes))
	}
}

func TestLoad_CustomPortEnv(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "api"
cmd = "echo"
port_env = "MY_PORT"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Processes[0].PortEnv != "MY_PORT" {
		t.Errorf("port_env: got %q, want MY_PORT", cfg.Processes[0].PortEnv)
	}
}

func TestLoad_Env(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "api"
cmd = "echo"
env = { FOO = "bar" }
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Processes[0].Env["FOO"] != "bar" {
		t.Errorf("env FOO: got %q", cfg.Processes[0].Env["FOO"])
	}
}

func TestLoad_MissingName(t *testing.T) {
	path := writeToml(t, `
[[process]]
cmd = "echo hello"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoad_MissingCmd(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "api"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing cmd")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/.invincible.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_RestartDelay_Default(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "api"
cmd = "echo"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Processes[0].RestartDelay != DefaultRestartDelay.String() {
		t.Errorf("default restart_delay: got %q, want %q", cfg.Processes[0].RestartDelay, DefaultRestartDelay.String())
	}
	if cfg.Processes[0].RestartDelayDuration() != DefaultRestartDelay {
		t.Errorf("RestartDelayDuration: got %v", cfg.Processes[0].RestartDelayDuration())
	}
}

func TestLoad_RestartDelay_Custom(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "api"
cmd = "echo"
restart_delay = "2s"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Processes[0].RestartDelayDuration().String() != "2s" {
		t.Errorf("restart_delay: got %v", cfg.Processes[0].RestartDelayDuration())
	}
}

func TestLoad_RestartDelay_Invalid(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "api"
cmd = "echo"
restart_delay = "not-a-duration"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid restart_delay")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	path := writeToml(t, `this is not valid toml ===`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoad_APIAddr(t *testing.T) {
	path := writeToml(t, `
[project]
name = "app"
api_addr = ":8888"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.APIAddr != ":8888" {
		t.Errorf("api_addr: got %q, want :8888", cfg.Project.APIAddr)
	}
}

func TestLoad_DependsOn(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "api"
cmd = "echo"
depends_on = ["worker"]

[[process]]
name = "worker"
cmd = "echo"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Processes[0].DependsOn) != 1 || cfg.Processes[0].DependsOn[0] != "worker" {
		t.Errorf("depends_on: got %v", cfg.Processes[0].DependsOn)
	}
}

func TestLoad_DependsOn_UnknownProcess(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "api"
cmd = "echo"
depends_on = ["ghost"]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
}

func TestLoad_DependsOn_Cycle(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "a"
cmd = "echo"
depends_on = ["b"]

[[process]]
name = "b"
cmd = "echo"
depends_on = ["a"]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for dependency cycle")
	}
}

func TestLoad_DependsOn_SelfCycle(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "a"
cmd = "echo"
depends_on = ["a"]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for self-dependency")
	}
}

func TestLoad_NoPort(t *testing.T) {
	path := writeToml(t, `
[[process]]
name = "api"
cmd = "echo"
no_port = true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Processes[0].NoPort {
		t.Error("no_port: expected true")
	}
}
