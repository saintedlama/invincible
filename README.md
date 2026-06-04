# Invincible

> **Proof of concept.** Invincible is an experiment, not a finished product. APIs, config format, and behaviour WILL change without notice.

A process manager for local development, written in Go.

Keep services alive, restart them on crash, assign free ports automatically, and wire up port dependencies between services — no Docker required. Comes with a terminal UI for humans and an HTTP API for agents.

## Invincible is Not

- A replacement for `make`
- A replacement for a full process manager like systemd or supervisor
- A hot-reload tool
- Intended for production use

## Installation

Requires Go 1.26 or later.

```sh
# Install to $GOPATH/bin
go install github.com/saintedlama/invincible/cmd/invincible@latest
```

## Building from source

```sh
# Or build from source
git clone https://github.com/saintedlama/invincible
cd invincible
go build -o invincible ./cmd/invincible
```

## Quick start

```sh
# Create a starter config in the current directory
invincible init

# Edit invincible.toml, then run
invincible
```

## Configuration

Invincible looks for `invincible.toml` in the current directory by default.

```toml
[project]
name    = "myapp"
# api_addr = ":7777"   # override the HTTP API port (default :7777)

[[process]]
name          = "api"
cmd           = "go run ./cmd/api"
port          = 8080          # hint; Invincible finds the next free port if taken
# port_env    = "PORT"        # env var injected with this process's port (default: PORT)
# no_port     = true          # disable port assignment for this process
# depends_on  = ["worker"]    # restart this process if a dependency changes port
# restart_delay = "500ms"     # wait before restarting after a crash (default: 500ms)
# env         = { QUEUE = "default" }  # extra static env vars

[[process]]
name = "worker"
cmd  = "go run ./cmd/worker"
# port omitted → Invincible assigns an arbitrary free port
```

### Port assignment

Every process that needs a port gets one automatically:

- If `port` is set, Invincible searches upward from that hint until it finds a free port.
- If `port` is omitted, the OS assigns an arbitrary free port.
- Set `no_port = true` to opt out entirely.

### Port environment variables

Before starting, Invincible injects into every process:

| Variable | Value |
|---|---|
| `PORT` (or `port_env`) | This process's assigned port |
| `<PEER_NAME>_PORT` | One variable per sibling that has a port, e.g. `API_PORT=8080` |

### Dependencies

```toml
[[process]]
name       = "frontend"
cmd        = "..."
depends_on = ["api"]
```

If `api` crashes and gets a **new port**, `frontend` is automatically restarted with the updated `API_PORT`. If the port doesn't change, `frontend` is left alone.

Cycles are detected at startup — Invincible exits with an error if a cycle exists.

## Running

```sh
invincible [flags]

Flags:
  --config    path to config file           (default: invincible.toml)
  --api-addr  preferred HTTP API address    (default: :7777, falls back to config api_addr)
  --no-tui    run headless, print API URL to stdout
```

### TUI key bindings

| Key | Action |
|---|---|
| `↑` / `k` | Select previous process |
| `↓` / `j` | Select next process |
| `s` | Start selected process |
| `x` | Stop selected process |
| `r` | Restart selected process |
| `Shift+↑` / `PgUp` | Scroll logs up |
| `Shift+↓` / `PgDn` | Scroll logs down |
| `q` / `Ctrl+C` | Quit |

## CLI commands

### `invincible init`

Create a starter `invincible.toml` in the current directory. Exits with an error if one already exists.

### `invincible skill`

Print an agent prompt (preamble + full skill text) to paste into an AI agent session. Requires `invincible.toml` to be present so the configured process list is included.

## HTTP API

The API binds to `127.0.0.1` and is only accessible locally. The default port is `7777`; if taken, an ephemeral port is used instead (printed to stdout in `--no-tui` mode).

| Method | Path | Description |
|---|---|---|
| `GET` | `/processes` | List all processes |
| `GET` | `/processes/{name}` | Get one process |
| `GET` | `/processes/{name}/logs` | Get recent logs (`?n=100&format=text`) |
| `POST` | `/processes/{name}/start` | Start a process |
| `POST` | `/processes/{name}/stop` | Stop a process |
| `POST` | `/processes/{name}/restart` | Restart a process |
| `GET` | `/openapi.json` | OpenAPI 3.0 spec |

### Process object

```json
{
  "Name":      "api",
  "State":     "running",
  "PID":       1234,
  "Cmd":       "go run ./cmd/api",
  "Port":      8080,
  "DependsOn": ["worker"],
  "Restarts":  0,
  "StartedAt": "2026-06-04T08:00:00Z",
  "Env":       { "QUEUE": "default" }
}
```

**States:** `stopped` · `starting` · `probing` · `running` · `crashed`

`probing` means the process has started but its port has not yet accepted a connection. Once the port is reachable the state transitions to `running`.

### Log entry object

```json
{ "time": "2026-06-04T08:22:01Z", "line": "server started on :8080", "stderr": false }
```

Use `?format=text` for plain newline-separated output (no metadata).
