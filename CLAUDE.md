# Invincible — Agent Initialization

> Read this file first when working on the Invincible codebase.

Invincible is a **local-development process manager** written in Go. It keeps services alive, restarts crashed processes, assigns free TCP ports, wires port dependencies, optionally rebuilds on file changes, and exposes both a terminal UI (TUI) and an HTTP API.

This repository is the Invincible project itself. It is also dogfooded: the repo contains `.invincible.toml`, so you can run `invincible` here to manage its own demo processes.

## Quick commands

```bash
# Build
make build

# Run tests
make test

# Run with coverage
make cover

# Format + vet + tidy + lint checks
make lint

# Run invincible in headless mode on this repo
./invincible --no-tui
```

## Repository layout

```
cmd/invincible/          # Cobra CLI entry points and commands
  main.go                # Calls Execute()
  root.go                # Root command: loads config, starts supervisor + API + TUI/watchers
  cmd_init.go            # `invincible init` — scaffolds .invincible.toml
  cmd_skill.go           # `invincible skill` / `invincible skill-spec`
  version.go             # Version flag helper

internal/
  config/                # TOML config parsing and validation
  supervisor/            # Process lifecycle: start, stop, restart, probing, logs, builds
  api/                   # HTTP API (chi) + OpenAPI spec
  graph/                 # Dependency graph: start/stop levels and cycle detection
  ports/                 # Free-port discovery and port probing
  watcher/               # fsnotify-based file watching + debounced rebuild/restart
  tui/                   # Bubble Tea terminal UI
```

## Architecture

### Startup flow (`cmd/invincible/root.go`)

1. Load config from `.invincible.toml` (`config.Load`).
2. Resolve API bind address: `--api-addr` flag → `project.api_addr` → path-derived offset from `:7777`.
3. Create `supervisor.New(processes)` and `api.New(sup, addr)`.
4. Write the actual bound address to `.invincible.port` next to the config file.
5. Start all processes in dependency order (`supervisor.StartAll`).
6. Start file watchers for any process with `watch` set. `build` is optional: if present, changes trigger a build then restart on success; if absent, changes restart the process directly.
7. Launch TUI, or run headless if `--no-tui`.

### Process lifecycle (`internal/supervisor`)

- `StateStopped` → `StateStarting` → (`StateProbing` if port) → `StateRunning`
- On unexpected exit: `StateCrashed`, then restart after `restart_delay`.
- `StateBuilding` is used during watch-triggered rebuilds; the old process keeps running until the build succeeds.
- Graceful shutdown sends SIGTERM to the process group, waits `shutdown_timeout`, then SIGKILL.

Port environment variables injected into each process:

- `PORT=<assigned>` (or `port_env` override)
- `<PEER_NAME>_PORT=<assigned>` for each sibling with a port

### Dependency model (`internal/graph`)

- `depends_on` declares that a process needs another process's port.
- The graph provides `StartLevels` (dependencies first) and `StopLevels` (dependents first).
- If a process restarts and receives a **new** port, dependents are cascade-restarted.
- Cycles are rejected at config load time.

### HTTP API (`internal/api`)

Binds `127.0.0.1` only. Endpoints:

| Method | Path | Description |
|---|---|---|
| GET | `/processes` | List all processes |
| GET | `/processes/{name}` | Get one process |
| GET | `/processes/{name}/logs?n=100&format=text` | Recent logs |
| POST | `/processes/{name}/start` | Start a process |
| POST | `/processes/{name}/stop` | Stop a process |
| POST | `/processes/{name}/restart` | Restart a process |
| POST | `/processes/restart-all` | Restart all processes |
| GET | `/openapi.json` | OpenAPI 3.0 spec |

Use `.invincible.port` to discover the current API address.

## Running Invincible on itself

The repo ships with a demo `.invincible.toml` that starts three Python http.server processes (`worker`, `api`, `frontend`) to demonstrate dependency wiring. To run it:

```bash
make build
./invincible --no-tui
```

Then in another terminal:

```bash
cat .invincible.port
curl -s http://$(cat .invincible.port)/processes | jq .
```

## Coding conventions

- Go 1.26+.
- Use `make lint` before committing; it runs `gofmt`, `go mod tidy`, `go vet`, `deadcode`, and `staticcheck`.
- Keep packages small and focused. `supervisor` is intentionally the single owner of process state.
- The TUI and API consume `supervisor` through small interfaces so they can be tested in isolation.
- Windows and Unix process-group handling are split into `proc_windows.go` and `proc_unix.go`.

## Common agent tasks

### Adding a config field

1. Add the field to `config.ProcessConfig` with TOML tags.
2. Apply defaults/validation in `config.Load`.
3. Use `ProcessConfig` helper methods for duration fields.
4. Add tests in `config/config_test.go`.

### Changing process lifecycle behavior

- Most logic lives in `internal/supervisor/supervisor.go`.
- `startProcess`, `stopProcess`, `Restart`, and `watch` are the key functions.
- Be careful with mutex ordering: `Supervisor.mu` protects the map; `process.mu` protects per-process state.

### Adding an API endpoint

1. Add the route in `internal/api/api.go`.
2. Update `internal/api/spec.go` for OpenAPI parity.
3. Add tests in `internal/api/api_test.go` using the existing test harness.

### Adding TUI behavior

- `internal/tui/tui.go` is the main model.
- Split rendering helpers into `process_list.go`, `process_detail.go`, `log.go`, and `help.go`.
- Bubble Tea v2 APIs differ from v1; refer to the vendored module if unsure.

## Things to avoid

- Do not bind the API to `0.0.0.0`; it is intentionally localhost-only.
- Do not break the `.invincible.port` contract: write on startup, remove on clean shutdown.
- Do not store absolute or platform-specific paths in config defaults.
- Do not change the state machine ordering without updating TUI color mapping and README.

## Useful references

- `README.md` — user-facing docs, config examples, TUI key bindings.
- `cmd/invincible/cmd_skill.go` — agent skill text; keep in sync if API/process schema changes.
- `C:\Users\chris\.agents\skills\invincible\SKILL.md` — installed agent skill (update if behavior changes).
