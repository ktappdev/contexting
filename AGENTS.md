# AGENTS.md — Contributing to Contexting

This file is for developers and AI agents working on the codebase. For user-facing documentation, see README.md.

## Description

**Contexting** is a Go CLI that pre-computes a rich index of a codebase so AI agents (and humans) can locate files via ranked search hints. It walks the filesystem once, extracts code symbols (functions/classes/types) statically, generates LLM synonyms for filenames, and persists a JSON tree at `.ctx/ctx_index.json`. A live `watch` mode keeps an in-memory copy fresh and serves search queries over a localhost HTTP endpoint, bypassing disk I/O for sub-second results.

## Building & Installing

| Command | Description |
|---------|-------------|
| `go install .` | Installs to `$GOPATH/bin` (uses hardcoded version) |
| `make build` | Builds to `bin/contexting` with version injected |
| `make install` | Installs with git-tag-based version via ldflags |
| `go test ./...` | Run all tests |

## Versioning

- **Hardcoded fallback**: `main.go` line ~9: `var version = "0.0.1"`
- **`make install`**: Overrides via `-ldflags "-X main.version=..."` using `git describe --tags`
- **Without tags**: Falls back to commit hash (e.g., `af85edb-dirty`)

### Bumping Version

```bash
git tag v0.0.2 && make install
contexting version  # → 0.0.2
```

## Project Structure

### CLI Entry Point
- `main.go` — Entry point, declares `var version`
- `commands.go` — Root cobra command, subcommand registration

### Commands
- `command_init.go` — `contexting init`
- `command_watch.go` — `contexting watch` (fsnotify-based)
- `command_search.go` — `contexting search-hints`
- `command_eval.go` — `contexting eval`
- `command_doctor.go` — `contexting doctor`
- `command_config.go` — `contexting config`

### Core Logic
- `indexer.go` — BuildIndex, parallel synonym+symbol goroutines
- `openrouter.go` — LLM synonym generation, batch processing
- `symbols.go` — Symbol extraction (Go/Python/JS/TS/Rust/Ruby)
- `search.go` — Search scoring logic
- `node.go` — Node data model (Full_path, Type, Symbols, Synonyms, Children)
- `node_mutation.go` — Upsert/remove nodes (used by watch)

### Watch Mode
- `index_manager.go` — IndexManager for watch mode
- `watch_helpers.go` — fsnotify watch setup, event filtering
- `memory_search_server.go` — HTTP endpoint for live search during watch

### Config & Files
- `config.go` — Config structs and loading
- `config_bootstrap.go` — Starter config creation, interactive prompts
- `ignore.go` — Ignore pattern matching (defaults + .gitignore + config)
- `io_atomic.go` — Atomic file writes via temp+rename

### Logging
- `logger.go` — `logInfof` (stdout), `logWarnf`/`logErrorf` (stderr). No log levels, always prints.

## Key Conventions

| Convention | Details |
|------------|---------|
| CLI framework | [github.com/spf13/cobra](https://github.com/spf13/cobra) |
| Config format | TOML (`.ctx/ctx_config.toml`) |
| Output format | JSON (`.ctx/ctx_index.json`) |
| Atomic writes | Temp file + rename (see `io_atomic.go`) |
| No file locking | Between processes (init vs watch can conflict) |

### Flag Defaults

- `--verbose` (default: false) — gates steady-state change summaries
- `--search-log` (default: true) — gates search query logging

### Ignore System

Order: defaultIgnores → `.gitignore` → config ignore list (additive)

Default ignores: `.venv`, `site-packages`, `__pycache__`, `node_modules`, `.env*`, build artifacts, hidden paths (`.` prefixed).

### Testing

- Test files sit alongside source: `*_test.go`
- Run: `go test ./...`

## Key Conventions for AI Agents

1. **Never assume file locking** — `init` and `watch` can conflict
2. **Atomic writes only** — use temp+rename pattern in `io_atomic.go`
3. **Config precedence** — CLI flags > `.ctx/ctx_config.toml` > hardcoded defaults
4. **Config paths** — Relative paths resolve from config file location
5. **No log levels** — `logInfof`/`logWarnf`/`logErrorf` always print; use stderr for warnings/errors
6. **Watch behavior** — Snapshot persistence is shutdown-only (in-memory updates during runtime, flush on graceful shutdown)