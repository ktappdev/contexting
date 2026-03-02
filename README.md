# Contexting

Contexting keeps a live map of your codebase so agents can reason about paths without hunting through the filesystem manually. It builds a tree of every folder and file, attaches synonyms, and exposes ranked hints plus health tooling.

## Key features
- `init`: create a fresh `context.json` snapshot with optional OpenRouter synonyms
- `watch`: keep an in-memory index updated while the process runs and write snapshots only on graceful shutdown
- `search-hints`: query the index for ranked paths with explainable scores
- `eval`: benchmark hit@k/MRR for agent queries
- `doctor`: inspect config/index/cache health and get fixes

## Quick start
```bash
go install github.com/ktappdev/contexting@latest
cd your-repo
cp context.toml.example context.toml
export OPENROUTER_API_KEY="sk-or-v1-..."  # optional if you want live synonyms
contexting init .                  # one-time bootstrap
contexting watch .                 # keep the index in-memory + flush on shutdown
``` 

### Notes
- CLI flags override `context.toml`, which in turn falls back to hard-coded defaults.
- Relative paths in `context.toml` resolve from the config file location.
- `watch` respects `.gitignore`, and if that file is missing it creates a starter list that ignores `.venv`, `site-packages`, `__pycache__`, `node_modules`, `.env*`, build/user artifacts, etc.
- Hidden paths are skipped by default: any file or directory starting with `.` is excluded from indexing.

## Commands
### `contexting init`
Create a full snapshot in `context.json` and a synonym cache.
```
contexting init . --output context.json --synonym-cache .contexting_synonyms_cache.json
```
Useful flags:
- `--no-config-prompt` and `--create-config` keep it non-interactive in automation
- `--llm-model`, `--batch-size`, `--synonyms`, `--api-key`, `--ignore`
- It always rebuilds the entire tree; use it when you need a clean snapshot.

### `contexting watch`
Keeps the same index in memory and writes the snapshot only when you stop gracefully.
```
contexting watch . --debounce 750ms --verbose
```
Highlights:
- Snapshot persistence is shutdown-only (in-memory updates during runtime, flush on graceful shutdown)
- `--llm-on-watch`: enable live synonym enrichment (off by default for responsiveness)
- `--search-log` (default true): logs memory search requests in the watch stream
- `--search-log-query-max` (default 120): truncates logged query text for readability
- `--create-config`/`--no-config-prompt`: control config creation
- Events are applied via a single worker, and logs show the changed files per cycle
- Starts a local memory-search endpoint and writes `.contexting_runtime.json` for discovery
- Example query log line: `2026-03-02 10:12:03 [INFO] Search query "auth middleware routes..." -> 5 results in 3ms`

### `contexting search-hints`
Ask for the best matching paths without scanning the repo.
```
contexting search-hints "update storage" --json
```
Flags:
- `--limit`, `--min-score`, `--type files|dirs|all`
- `--dir-summary`, `--dir-limit`, `--drill-limit` for top-down directory-first results with per-directory drill-down hits
- `--explain`, `--show-tokens`, `--json`
- `--memory` (default true) queries the live in-memory watch index first
- `--memory-only` fails if live memory is unavailable
- `--runtime-file` points to runtime discovery file (default `.contexting_runtime.json` near index path)
- Falls back to `context.json` snapshot when memory endpoint is not running (unless `--memory-only` is set)
- Low-signal short/common words are filtered from query and synonym matching to reduce noisy results.

Example directory summary:
```
contexting search-hints "routing auth" --dir-summary --dir-limit 5 --drill-limit 3
```

### `contexting eval`
Measure Hit@1/3/5 + MRR from manual query cases.
```
contexting eval --cases eval_cases.json --json
```
Input format:
```json
[
  {"query": "auth middleware", "expect_any": ["internal/auth/middleware.go"]}
]
```

### `contexting doctor`
Health-check your config/root/index/cache/API key.
```
contexting doctor --json
```
It reports pass/warn/fail reasons plus suggestions.

### `contexting config init`
Create or overwrite `context.toml`:
```
contexting config init --output context.toml
```

## Data flow
1. `init` or load `context.json` → builds a tree with synonyms
2. `watch` keeps that tree in RAM; events mutate only memory
3. Snapshot is flushed on graceful shutdown
4. `search-hints` and `eval` load the latest JSON or call the CLI for agent flows

## File formats
- `context.json`: root path, timestamp, tree with `full_path`, `type`, `synonyms`, and `children`
- `.contexting_synonyms_cache.json`: basename → synonyms cache for reuse
- `context.toml`: config-driven defaults (see `context.toml.example`)
  - `watch.search_log`, `watch.search_log_query_max`
  - `search.dir_summary`, `search.dir_limit`, `search.drill_limit`
  - `common.ignore` starter defaults include `.venv`, `site-packages`, and `__pycache__`

## Testing
```bash
go test ./...
```

## Troubleshooting
- Run `contexting doctor --json` for diagnostics
- If `context.json` is stale, restart watch or run `contexting init`
- If you changed ignore rules (for example `.venv`/`site-packages`), run `contexting init` or restart `watch` to rebuild the in-memory/snapshot index.
- To disable live LLM work use `--llm-on-watch=false` or remove `watch.llm` from config
