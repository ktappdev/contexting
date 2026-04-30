# Contexting

Contexting keeps a live map of your codebase so AI agents can reason about paths without hunting through the filesystem manually. It builds a recursive JSON tree of every folder and file, extracts code symbols (functions, classes, types, variables) using language-specific extractors, attaches LLM-generated synonyms, and exposes ranked search hints plus health tooling.

## Quick start

```bash
go install github.com/ktappdev/contexting@latest
cd your-repo
contexting init .
contexting watch .
```

Set an API key for synonym generation (optional but recommended):

```bash
export OPENROUTER_API_KEY="sk-or-v1-..."
```

First run creates a `context.toml` config file. Edit it, then press Enter to continue. Subsequent runs use the saved config.

## How it works

### Index construction

`contexting init` walks the filesystem tree and builds `context.json` — a single recursive JSON tree where every entry has `full_path`, `type`, optional `symbols`, optional `synonyms`, and nested `children` for directories.

**Symbols:** Every source file gets scanned with language-specific extractors that pull out exported symbols — functions, types, variables, classes, constants. These become a `symbols` array on the file node.

**Synonyms:** Each node (file or directory) gets 4–8 LLM-generated synonyms via batched API calls to an OpenRouter-compatible endpoint. For example, `Skeletons.tsx → ["skeletons", "loading", "placeholder", "animation"]`. Names are batched (default 15 per request, 4 parallel) and processed concurrently.

**Bootstrap diff:** On subsequent runs, `init` diffs the filesystem against the existing snapshot using file modification times. Deleted files are removed, new files are added, modified files are re-extracted. No LLM calls for existing synonyms — run `contexting sync` to fill gaps.

### Search mechanics

When you run `search-hints "product review rating"`, the query is lowercased and split into tokens. Each token is scored against every node in the tree using matchers that each contribute points:

| Matcher | Points | Description |
|---------|--------|-------------|
| exact | +12 | Token exactly matches a directory basename |
| basename | +7 | Token matches the file's basename |
| path segment | +4 | Token matches any segment in the full path |
| segment prefix | +5 | Token is a prefix of a path segment |
| syn-exact | +8 | Token exactly matches a synonym |
| syn overlap | +5 | Token partially overlaps a synonym |
| sym exact | +8 | Token exactly matches a symbol name |
| sym contains | +5 | Token is contained in a symbol name |
| sym token | +4 | Token matches a camelCase/PascalCase split token in a symbol |

The `--explain` flag reveals this breakdown:

```
basename contains +7: skeleton + syn exact +8: loading + sym contains +5: PageSkeleton = total 148
```

Results are ranked by total score. Low-signal short/common words are filtered from query and synonym matching to reduce noise.

### Watch mode

`contexting watch` runs as a daemon, maintaining the index in memory. It watches for filesystem changes via debounced events (750ms default), re-extracts symbols for modified files, and serves a `.contexting_runtime.json` that `search-hints --memory` reads for live results. The on-disk snapshot is flushed every 45s (`persist_interval`) and on graceful shutdown. This gives sub-second updates during active development.

## Commands

### `contexting init`

Create a full snapshot in `context.json` with extracted symbols and optional synonyms.

```bash
contexting init .
contexting init . --output context.json --synonym-cache .contexting_synonyms_cache.json
```

Key flags:
- `--no-config-prompt`, `--create-config` — non-interactive automation
- `--llm-model`, `--batch-size`, `--synonyms-min`, `--synonyms-max`, `--api-key`, `--ignore`
- `-v, --verbose` — show symbol extraction progress and batch completion
- Always rebuilds the entire tree; use when you need a clean snapshot

On first run, creates a starter `context.toml` and pauses so you can edit it.

### `contexting sync`

Targeted synonym generation — only generates synonyms for names that are missing or below `synonyms_min`. Works on the existing index without rebuilding.

```bash
contexting sync .
```

Use after `init` to fill synonym gaps, or after adding new files to catch names that were missed.

### `contexting watch`

Keep the index in memory with live filesystem updates.

```bash
contexting watch . --debounce 750ms --verbose
```

Key flags:
- `--llm-on-watch` (default true) — live synonym enrichment for new files
- `--search-log` (default true) — log memory search requests
- `--search-log-query-max` (default 120) — truncate logged queries
- `--persist` (default "shutdown") — when to flush: `shutdown`, `interval`, `never`
- `--persist-interval` (default "45s") — flush interval when persist=interval
- Starts a local memory-search endpoint and writes `.contexting_runtime.json`
- Events applied via a single worker; logs show changed files per cycle

### `contexting search-hints`

Query the index for ranked paths with explainable scores.

```bash
contexting search-hints "update storage" --json
contexting search-hints "routing auth" --dir-summary --dir-limit 5 --drill-limit 3
```

Flags:
- `--limit`, `--min-score`, `--type files|dirs|all`
- `--dir-summary`, `--dir-limit`, `--drill-limit` — top-down directory-first results
- `--explain`, `--show-tokens`, `--json`
- `--memory` (default true) — query live watch index first, fall back to snapshot
- `--memory-only` — fail if live memory unavailable
- `--runtime-file` — path to runtime discovery file (default `.contexting_runtime.json`)

### `contexting eval`

Benchmark Hit@1/3/5 + MRR from manual query cases.

```bash
contexting eval --cases eval_cases.json --json
```

Input format:
```json
[
  {"query": "auth middleware", "expect_any": ["internal/auth/middleware.go"]}
]
```

### `contexting doctor`

Health-check config, root, index, cache, and API key.

```bash
contexting doctor --json
```

### `contexting config init`

Create or overwrite `context.toml`:

```bash
contexting config init --output context.toml
```

## Configuration

`context.toml` drives all defaults. CLI flags override config, which overrides hard-coded defaults.

```toml
[common]
output = "context.json"
synonym_cache = ".contexting_synonyms_cache.json"
llm_model = "meta-llama/llama-3.1-8b-instruct"
batch_size = 15              # names per LLM request
synonyms_min = 4             # min synonyms per name
synonyms_max = 8             # max synonyms per name
ignore = [".git", ".venv", "site-packages", "__pycache__", "node_modules", "vendor", "dist", "migrations", "pb_migrations", "alembic", "flyway"]
dot_whitelist = []           # extra dot files to keep (merged with built-in defaults)
verbose = true

[llm]
provider = "openrouter"
endpoint = "https://openrouter.ai/api/v1/chat/completions"
model = "meta-llama/llama-3.1-8b-instruct"
api_key_env = "OPENROUTER_API_KEY"   # reads key from env var
temperature = 0.3
parallel_requests = 4        # concurrent LLM batches
```

**LLM config resolution:** flag → config `api_key` → config `api_key_env` → `LLM_API_KEY` → `OPENROUTER_API_KEY`.

**Supported providers:** Any OpenAI-compatible API — OpenRouter (default), OpenAI, Anthropic, local endpoints. Set `provider`, `endpoint`, and `model` in `[llm]`.

## Ignore system

Contexting respects `.gitignore` by default. Additional ignores come from:

1. **Built-in defaults** — `.git`, `.venv`, `site-packages`, `__pycache__`, `node_modules`, `vendor`, `dist`, `migrations`, `pb_migrations`, `alembic`, `flyway`
2. **`.gitignore` patterns** — loaded from the project root
3. **`ignore` in config** — extra patterns merged with defaults

**Dot files** are skipped by default (any path segment starting with `.`). whitelisted dot files (`.env`, `.prettierrc`, `.editorconfig`, etc.) are kept. Add more via `dot_whitelist` in config.

## Data flow

```
init → walk filesystem → extract symbols → (LLM: generate synonyms) → context.json
                                                                    ↓
watch → load context.json → keep in RAM → filesystem events → mutate in-memory
                                                                    ↓
search-hints → load context.json (or query live memory) → score tokens → ranked results
```

## File formats

- **`context.json`** — root path, timestamp, tree with `full_path`, `type`, `symbols`, `synonyms`, `children`
- **`.contexting_synonyms_cache.json`** — basename → synonyms cache for reuse across runs
- **`context.toml`** — config-driven defaults
- **`.contexting_runtime.json`** — live watch discovery for memory search

## Project size guard

Projects with more than ~9 synonym batches (>135 names at batch_size=15) trigger a warning. LLM reliability drops at scale. Use `contexting sync` for targeted generation on large projects.

## Testing

```bash
go test ./...
```

## Troubleshooting

- `contexting doctor --json` for diagnostics
- If `context.json` is stale, restart watch or run `contexting init`
- If you changed ignore rules, run `contexting init` or restart `watch` to rebuild
- Synonym generation requires `OPENROUTER_API_KEY` or `--api-key`. Disable with `--llm-on-watch=false` or `watch.llm = false`
- Watch mode must be stopped gracefully (Ctrl+C) to flush the snapshot
