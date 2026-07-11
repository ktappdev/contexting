# AGENTS.md — Contributing to Contexting

**The command is `ctxt` — short for Contexting.**

This file is for developers and AI agents working on the codebase. For user-facing documentation, see README.md.

## Description

**Contexting** is a Go CLI that pre-computes a rich index of a codebase so AI agents (and humans) can locate files via ranked search hints. It walks the filesystem once, extracts code symbols (functions/classes/types) statically, generates LLM synonyms for filenames, and persists a JSON tree at `.ctxt/ctx_index.json`. A live `watch` mode keeps an in-memory copy fresh and serves search queries over a localhost HTTP endpoint, bypassing disk I/O for sub-second results.

## Building & Installing

| Command | Description |
|---------|-------------|
| `go install ./cmd/ctxt` | Installs to `$GOPATH/bin` (uses hardcoded version) |
| `make build` | Builds to `bin/ctxt` with version injected |
| `make install` | Installs `ctxt` with git-tag-based version via ldflags |
| `go test ./...` | Run all tests |

## Versioning

- **Hardcoded fallback**: `main.go` line ~3: `var Version = "0.0.1"`
- **`make install`**: Overrides via `-ldflags "-X github.com/ktappdev/contexting.Version=..."` using `git describe --tags`
- **Without tags**: Falls back to commit hash (e.g., `af85edb-dirty`)

### Bumping Version

```bash
git tag v0.0.2 && make install
ctxt version  # → 0.0.2
```

## Project Structure

### CLI Entry Point
- `cmd/ctxt/main.go` — Entry point, calls `contexting.NewRootCommand().Execute()`
- `main.go` — Library package root, declares `var Version`
- `commands.go` — Root cobra command, subcommand registration

### Commands
- `command_init.go` — `ctxt init`
- `command_watch.go` — `ctxt watch` (fsnotify-based)
- `command_search.go` — `ctxt search-hints`
- `command_eval.go` — `ctxt eval`
- `command_bench.go` — `ctxt bench` (compare search engines)
- `command_doctor.go` — `ctxt doctor`
- `command_config.go` — `ctxt config`
- `command_status.go` — `ctxt status` (index health report)
- `command_clean.go` — `ctxt clean` (remove `.ctxt/` directory)
- `command_sync.go` — `ctxt sync` (targeted synonym generation)
- `command_shared.go` — shared command flags/helpers

### Core Logic
- `indexer.go` — BuildIndex, sequential symbol extraction then synonym generation (symbols fed to LLM)
- `openrouter.go` — LLM synonym generation with symbol context, batch processing, conceptual synonym prompt
- `symbols.go` — Symbol extraction (Go/Python/JS/TS/Rust/Ruby)
- `search.go` — Search scoring logic
- `node.go` — Node data model (Full_path, Type, Symbols, Synonyms, Children)
- `node_mutation.go` — Upsert/remove nodes (used by watch)
- `bench_engine.go` — SearchEngine interface + ctxt/find/fd/grep/rg/hybrid/combined implementations
- `bench_report.go` — bench report formatting (table, JSON, category-grouped)
- `command_shared.go` — shared flags, `normalize()`, `resolveLLMConfig()`
- `config_apply.go` — `applyCommonConfig` from TOML to flags
- `config_template.go` — config template generation
- `paths.go` — path resolution utilities
- `runtime_state.go` — runtime state for watch mode discovery
- `cache.go` — synonym cache load/save
- `storage.go` — storage helpers
- `errors.go` — error types
- `signals.go` — signal handling for graceful shutdown
- `synonyms.go` — synonym sanitization, lexical fallback, token splitting
- `search_summary.go` — directory summary formatting
- `memory_search_client.go` — client for live memory search
- `agent_demo.go` — agent demo

### Watch Mode
- `index_manager.go` — IndexManager for watch mode
- `watch_helpers.go` — fsnotify watch setup, event filtering
- `memory_search_server.go` — HTTP endpoint for live search during watch

### Config & Files
- `config.go` — Config structs and loading
- `config_bootstrap.go` — Starter config creation, interactive prompts
- `config_template.go` — config template generation with current defaults
- `config_apply.go` — apply config values to flags
- `ignore.go` — Ignore pattern matching (defaults + .gitignore + config)
- `io_atomic.go` — Atomic file writes via temp+rename

### Logging
- `logger.go` — `LogInfof` (stdout), `LogWarnf`/`LogErrorf` (stderr). No log levels, always prints.

## Key Conventions

| Convention | Details |
|------------|---------|
| CLI framework | [github.com/spf13/cobra](https://github.com/spf13/cobra) |
| Config format | TOML (`.ctxt/ctx_config.toml`) |
| Output format | JSON (`.ctxt/ctx_index.json`) |
| Atomic writes | Temp file + rename (see `io_atomic.go`) |
| No file locking | Between processes (init vs watch can conflict) |
| Default model | `deepseek/deepseek-v4-flash` |
| Synonyms | 5–12 per name |
| Temperature | `0.9` |
| Parallel requests | `10` |
| Symbol flow | Symbols extracted BEFORE synonyms (sequential), then fed to LLM during synonym generation |
| Default bench engines | `ctxt,find,grep` (`fd`, `rg`, `hybrid`, `combined` are opt-in via `--engines`) |
| Scoring | Exact basename match +15 (highest); confidence gap heuristic truncates results at score cliffs |
| `--agent` flag | Blocks state-modifying commands (`init`/`watch`/`sync`/`clean`) |

### Flag Defaults

- `--verbose` (default: false) — gates steady-state change summaries
- `--search-log` (default: true) — gates search query logging
- `--by-category` (default: true) — group bench results by category
- `--engines` (default: `"ctxt,find,grep"`) — engines to benchmark. Also available: `fd`, `rg`, `hybrid`, `combined`
- `--hybrid` (search flag, default: false) — enable content fallback via ripgrep when index results are sparse
- `--hybrid-score` (search flag, default: 1) — score assigned to content-matched results
- `--synonyms-min` (default: `5`) — min synonyms per name
- `--synonyms-max` (default: `12`) — max synonyms per name

### Ignore System

Order: defaultIgnores → `.gitignore` → config ignore list (additive)

Default ignores: `.venv`, `site-packages`, `__pycache__`, `node_modules`, `.env*`, build artifacts, hidden paths (`.` prefixed).

### Testing

- Test files sit alongside source: `*_test.go`
- Run: `go test ./...`

## Key Conventions for AI Agents

1. **Naming — The CLI binary and command is `ctxt` (lowercase), not `contexting`. The project name is Contexting (capitalized). All user-facing command strings, error messages, help text, MCP server names, bench engine names, and doc examples MUST use `ctxt`. Only use `Contexting` when referring to the project in prose.**
2. **Never assume file locking** — `init` and `watch` can conflict
3. **Atomic writes only** — use temp+rename pattern in `io_atomic.go`
4. **Config precedence** — CLI flags > `.ctxt/ctx_config.toml` > hardcoded defaults
5. **Config paths** — Relative paths resolve from config file location
6. **No log levels** — `LogInfof`/`LogWarnf`/`LogErrorf` always print; use stderr for warnings/errors
7. **Watch behavior** — Snapshot persistence is shutdown-only (in-memory updates during runtime, flush on graceful shutdown)
8. **Symbol-feeding** — Symbols extracted before synonyms, passed to LLM for domain-accurate generation
9. **Bench case format** — v2 supports categories, v1 still works, `LoadCasesAuto` handles both
10. **Scoring** — Exact basename +15 (highest), confidence gap truncation reduces noise
11. **Hybrid search** — Use `--hybrid` on `search-hints` to fill gaps in index results. Ripgrep scans file contents for query tokens and merges unmatched files at score=1. Useful when the index misses files whose content literally contains the query words but whose symbols/synonyms don't connect. Add `--memory=false` if using a snapshot (non-watch) index.
12. **`--agent` flag** — Blocks `init`/`watch`/`sync`/`clean` for safety in automated flows

## Bench Engines

| Engine | What it does | Use case |
|--------|--------------|----------|
| **ctxt** | Precomputed index — symbols, synonyms, path scoring. Ranked. | Default. When you want relevance-ranked results. |
| **find** | Unix `find` — lists all files, filters by name tokens. Alphabetical. | Baseline filename search. |
| **fd** | Faster `find` alternative — respects `.gitignore`. Alphabetical. | Gitignore-aware filename baseline. |
| **grep** | Unix `grep` — searches file contents for tokens. Alphabetical. | Baseline content search, no gitignore. |
| **rg** | ripgrep — faster `grep`, respects `.gitignore`. Alphabetical. | Gitignore-aware content baseline. |
| **hybrid** | ctxt + content fallback via ripgrep. Content matches at score=1. | When index might have blind spots. Slower (7ms avg). |
| **combined** | Union of find + grep. Alphabetical. | Maximum recall at cost of noise. |

Run: `ctxt bench --cases docs/bench_cases.json --engines ctxt,hybrid,find,grep,rg`