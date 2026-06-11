package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/saintedlama/invincible/internal/config"
)

var skillCmd = &cobra.Command{
	Use:          "skill",
	Short:        "Print an agent prompt for installing the Invincible skill for this project",
	SilenceUsage: true,
	RunE:         runSkill,
}

func init() {
	rootCmd.AddCommand(skillCmd)
}

func runSkill(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("no .invincible.toml found in the current directory — run `invincible init` to create one")
	}

	fmt.Print("Paste the text below into your agent prompt to have the agent install an Invincible skill for this project.\n\n")

	fmt.Print(`Install a skill named "invincible" with the following description and API spec:

# Invincible — process manager skill

Invincible is a local-development process manager. It keeps services alive,
restarts them on crash, assigns free ports, and exposes an HTTP API you can
use to inspect and control processes programmatically.

## Finding the API address

Primary: Read .invincible.port in the project root. It contains the bound
address (e.g. 127.0.0.1:7777) and is written on startup, removed on shutdown.

When no explicit api_addr is configured, Invincible derives a deterministic
port from the project directory path (hash-based offset from 7777). This means
each worktree gets its own preferred port, avoiding collisions.

Override in config:  api_addr = ":8888"   (under [project])
Override at runtime: invincible --api-addr :8888

The API always binds 127.0.0.1. If the preferred port is taken it falls back
to an OS-assigned ephemeral port.

When multiple invincible instances run (e.g. across git worktrees), each has its
own .invincible.port file — always read it from the project root you are working in.

## Endpoints

### List all processes
GET /processes
→ array of ProcessStatus (see schema below)

### Get one process
GET /processes/{name}
→ ProcessStatus or 404

### Get logs
GET /processes/{name}/logs?n=100&format=text
  n       – number of recent lines (default 100)
  format  – omit for JSON, use "text" for plain newline-separated output

JSON response: array of log entries
  { "time": "2026-06-04T08:22:01Z", "line": "server started", "stderr": false }

Plain-text response: one line per entry, no metadata (convenient for curl).

### Control a process
POST /processes/{name}/start
POST /processes/{name}/stop
POST /processes/{name}/restart
→ { "ok": "true" } or 500 with error message

## ProcessStatus schema

{
  "Name":      "api",
  "State":     "running",       // stopped | starting | probing | running | crashed
  "PID":       1234,            // 0 when not running
  "Cmd":       "go run ./cmd/api",
  "Port":      8080,            // assigned port (0 if no_port = true)
  "Restarts":  2,               // crash-triggered restart count
  "StartedAt": "2026-06-04T08:00:00Z",
  "Env":       { "QUEUE": "default" }
}

## Process states

stopped  – not running, either never started or deliberately stopped
starting – OS process launched, before port check
probing  – process is up but its port has not accepted a connection yet
running  – process is up and port (if any) is accepting connections
crashed  – exited unexpectedly; will be restarted after restart_delay

## Port environment variables

Invincible injects these into every process environment:
  PORT=<n>              – this process's own assigned port (name via port_env)
  <PEER_NAME>_PORT=<n>  – one var per sibling that has a port, e.g. API_PORT=8080

## Typical agent workflows

Check if a service is healthy:
  GET /processes/{name}  →  assert State == "running"

Tail logs after a restart:
  GET /processes/{name}/logs?n=50&format=text

Restart a crashed service:
  POST /processes/{name}/restart

Wait for a service to become ready (poll until running):
  loop: GET /processes/{name}  →  break when State == "running"

Discover which port a service was assigned:
  GET /processes/{name}  →  read Port field

## OpenAPI spec

GET /openapi.json  →  full OpenAPI 3.0 spec
`)

	fmt.Print("\n## Processes configured in this project\n\n")
	for _, p := range cfg.Processes {
		port := "auto-assigned"
		if p.NoPort {
			port = "none"
		} else if p.Port > 0 {
			port = fmt.Sprintf("%d", p.Port)
		}
		fmt.Printf("  %-16s port: %-14s cmd: %s\n", p.Name, port, p.Cmd)
	}
	fmt.Println()
	return nil
}
