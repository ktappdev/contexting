# Contexting

Contexting keeps a live map of your codebase so agents can reason about paths without hunting through the filesystem manually. It builds a tree of every folder and file, attaches synonyms, and exposes ranked hints plus health tooling.

## Key features
- `init`: create a fresh `context.json` snapshot with optional OpenRouter synonyms
- `watch`: keep an in-memory index updated while the process runs and only write snapshots when you stop (or on a configurable interval)
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
- `watch` respects `.gitignore`, and if that file is missing it creates a starter list that ignores `node_modules`, `.env*`, build/users artifacts, etc.

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
Keeps the same index in memory and writes the snapshot only when you stop (default) or periodically.
```
contexting watch . --persist shutdown --debounce 750ms --verbose
```
Highlights:
- `--persist shutdown`: in-memory index is flushed to disk only on graceful shutdown (default)
- `--persist interval --persist-interval 45s`: sprinkle in periodic saves
- `--llm-on-watch`: enable live synonym enrichment (off by default for responsiveness)
- `--create-config`/`--no-config-prompt`: control config creation
- Events are applied via a single worker, and logs show the changed files per cycle
- Starts a local memory-search endpoint and writes `.contexting_runtime.json` for discovery

### `contexting search-hints`
Ask for the best matching paths without scanning the repo.
```
contexting search-hints "update storage" --json
```
Flags:
- `--limit`, `--min-score`, `--type files|dirs|all`
- `--explain`, `--show-tokens`, `--json`
- `--memory` (default true) queries the live in-memory watch index first
- `--memory-only` fails if live memory is unavailable
- `--runtime-file` points to runtime discovery file (default `.contexting_runtime.json` near index path)
- Falls back to `context.json` snapshot when memory endpoint is not running (unless `--memory-only` is set)

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
3. Snapshot is flushed on shutdown (persist=shutdown) or per interval (persist=interval)
4. `search-hints` and `eval` load the latest JSON or call the CLI for agent flows

## File formats
- `context.json`: root path, timestamp, tree with `full_path`, `type`, `synonyms`, and `children`
- `.contexting_synonyms_cache.json`: basename → synonyms cache for reuse
- `context.toml`: config-driven defaults (see `context.toml.example`)

## Testing
```bash
go test ./...
```

## Troubleshooting
- Run `contexting doctor --json` for diagnostics
- If `context.json` is stale, restart watch or run `contexting init`
- To disable live LLM work use `--llm-on-watch=false` or remove `watch.llm` from config
