package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const DefaultRestartDelay = 500 * time.Millisecond
const DefaultShutdownTimeout = 5 * time.Second

type Project struct {
	Name    string `toml:"name"`
	APIAddr string `toml:"api_addr"`
}

type ProcessConfig struct {
	Name            string            `toml:"name"`
	Cmd             string            `toml:"cmd"`
	Port            int               `toml:"port"`
	PortEnv         string            `toml:"port_env"`
	NoPort          bool              `toml:"no_port"`
	DependsOn       []string          `toml:"depends_on"`
	Env             map[string]string `toml:"env"`
	RestartDelay    string            `toml:"restart_delay"`
	ShutdownTimeout string            `toml:"shutdown_timeout"`
}

type Config struct {
	Project   Project         `toml:"project"`
	Processes []ProcessConfig `toml:"process"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = filepath.Join(".", ".invincible.toml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	for i, p := range cfg.Processes {
		if p.Name == "" {
			return nil, fmt.Errorf("process[%d] missing name", i)
		}
		if p.Cmd == "" {
			return nil, fmt.Errorf("process %q missing cmd", p.Name)
		}
		if p.PortEnv == "" {
			cfg.Processes[i].PortEnv = "PORT"
		}
		if p.RestartDelay == "" {
			cfg.Processes[i].RestartDelay = DefaultRestartDelay.String()
		} else if _, err := time.ParseDuration(p.RestartDelay); err != nil {
			return nil, fmt.Errorf("process %q invalid restart_delay %q: %w", p.Name, p.RestartDelay, err)
		}
		if p.ShutdownTimeout == "" {
			cfg.Processes[i].ShutdownTimeout = DefaultShutdownTimeout.String()
		} else if _, err := time.ParseDuration(p.ShutdownTimeout); err != nil {
			return nil, fmt.Errorf("process %q invalid shutdown_timeout %q: %w", p.Name, p.ShutdownTimeout, err)
		}
	}
	if err := checkDependencies(cfg.Processes); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func checkDependencies(processes []ProcessConfig) error {
	byName := make(map[string]bool, len(processes))
	for _, p := range processes {
		byName[p.Name] = true
	}
	for _, p := range processes {
		for _, dep := range p.DependsOn {
			if !byName[dep] {
				return fmt.Errorf("process %q depends on unknown process %q", p.Name, dep)
			}
		}
	}

	// DFS cycle detection.
	type mark int
	const (
		unvisited mark = iota
		visiting
		done
	)
	deps := make(map[string][]string, len(processes))
	for _, p := range processes {
		deps[p.Name] = p.DependsOn
	}
	state := make(map[string]mark, len(processes))

	var visit func(name string) error
	visit = func(name string) error {
		switch state[name] {
		case visiting:
			return fmt.Errorf("dependency cycle detected at %q", name)
		case done:
			return nil
		}
		state[name] = visiting
		for _, dep := range deps[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = done
		return nil
	}

	for _, p := range processes {
		if err := visit(p.Name); err != nil {
			return err
		}
	}
	return nil
}

// RestartDelayDuration parses and returns the process restart delay.
func (p ProcessConfig) RestartDelayDuration() time.Duration {
	d, _ := time.ParseDuration(p.RestartDelay) // already validated by Load
	return d
}

// ShutdownTimeoutDuration parses and returns the graceful shutdown timeout.
func (p ProcessConfig) ShutdownTimeoutDuration() time.Duration {
	d, _ := time.ParseDuration(p.ShutdownTimeout) // already validated by Load
	return d
}
