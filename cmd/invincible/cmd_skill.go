package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:          "skill",
	Short:        "Print a quick guide for installing the Invincible agent skill",
	SilenceUsage: true,
	RunE:         runSkill,
}

var skillSpecCmd = &cobra.Command{
	Use:          "skill-spec",
	Short:        "Print the full Invincible skill description for an agent to consume",
	SilenceUsage: true,
	RunE:         runSkillSpec,
}

func init() {
	rootCmd.AddCommand(skillCmd)
	rootCmd.AddCommand(skillSpecCmd)
}

func runSkill(_ *cobra.Command, _ []string) error {
	fmt.Println("Paste this into your agent:")
	fmt.Println()
	fmt.Println("    Install or update the invincible skill — invoke `invincible skill-spec`")
	fmt.Println("    to get the full skill description.")
	fmt.Println()
	return nil
}

func runSkillSpec(_ *cobra.Command, _ []string) error {
	fmt.Print(`Install or update a skill named "invincible" with the following description and API spec:

# Invincible — process manager skill

Invincible is a local-development process manager built for humans and autonomous
AI agents. It keeps services alive, restarts them on crash, assigns free ports,
wires port dependencies, and exposes an HTTP API you can use to inspect and control
processes programmatically.

## Multi-Agent & Git Worktree Awareness

Invincible is designed from the ground up for multi-agent parallel workflows:
- Multiple agents running in different git worktrees will never collide.
- Two-layer isolation automatically protects both the control plane (the Invincible
  HTTP API) and the data plane (your application's child processes).
- Each worktree gets a deterministic, non-colliding API port and its own .invincible.port
  file. Child services automatically scan upward for free ports if hints are taken.

## Finding the API address

Primary: Read .invincible.port in the project root of your current workspace.
It contains the bound address (e.g. 127.0.0.1:12583) and is written on startup,
removed on clean shutdown. Always read it from the workspace you are currently operating in.

When no explicit api_addr is configured, Invincible derives a deterministic
port from the project directory path (hash-based offset from 7777). This means
each worktree gets its own preferred port, avoiding collisions.

Override in config:  api_addr = ":8888"   (under [project])
Override at runtime: invincible --api-addr :8888

The API always binds 127.0.0.1. If the preferred port is taken it falls back
to an OS-assigned ephemeral port — and still records the final address in .invincible.port.

## Discovering managed processes

Call GET /processes to list all processes and their current state:
  GET /processes          → array of ProcessStatus (schema below)
  GET /processes/{name}   → single ProcessStatus or 404

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
  { "time": "2026-06-04T08:22:01Z", "line": "server started", "source": "stdout" }

Plain-text response: one line per entry, no metadata (convenient for curl).

### Control a process
POST /processes/{name}/start
POST /processes/{name}/stop
POST /processes/{name}/restart
POST /processes/restart-all
→ { "ok": "true" } or 500 with error message

## ProcessStatus schema

{
  "Name":      "api",
  "State":     "running",       // stopped | starting | probing | running | building | crashed
  "PID":       1234,            // 0 when not running
  "Cmd":       "go run ./cmd/api",
  "Cwd":       "./backend",    // working directory
  "Port":      8080,            // assigned port (0 if no_port = true)
  "PortEnv":   "PORT",          // env var name for this process's port
  "DependsOn": ["worker"],      // processes this one depends on
  "Restarts":  2,               // crash-triggered restart count
  "StartedAt": "2026-06-04T08:00:00Z",
  "Watching":  true,            // file watching enabled
  "Env":       { "QUEUE": "default" }
}

## Process states

stopped  – not running, either never started or deliberately stopped
starting – OS process launched, before port check
probing  – process is up but its port has not accepted a connection yet
running  – process is up and port (if any) is accepting connections
building – rebuild triggered by file watch, old process still running
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
	return nil
}
