# Contexting

**The command is `ctxt` — short for Contexting.**

Contexting keeps a live map of your codebase so AI agents can reason about paths without hunting through the filesystem manually. It builds a recursive JSON tree of every folder and file, extracts code symbols (functions, classes, types, variables) using language-specific extractors, attaches LLM-generated synonyms, and exposes ranked search hints plus health tooling.

## Quick start

```bash
go install github.com/ktappdev/contexting/cmd/ctxt@latest
cd your-repo
ctxt init .
ctxt watch .
```

Set an API key for synonym generation (optional but recommended):

```bash
export OPENROUTER_API_KEY="sk-or-v1-..."
```

First run creates a `.ctxt/ctx_config.toml` config file. Edit it, then press Enter to continue. Subsequent runs use the saved config.

## LLM flexibility

Synonyms are optional. Contexting works without an LLM — you get symbols and path matching, just no synonym expansion. With an LLM, search gets a significant boost since synonyms bridge the gap between how code is named and how developers talk about it.

The default uses OpenRouter with `deepseek/deepseek-v4-flash` — fast, nearly free (~$0.0004 per 60 names). But any OpenAI-compatible API works:

- **OpenRouter** (default) — access to dozens of models, free tier available
- **Local** — point `endpoint` at any local server (Ollama, llama.cpp, vLLM)
- **OpenAI / Anthropic** — set `provider`, `endpoint`, and `api_key`
- **No LLM** — skip the `[llm]` section entirely or set `api_key = ""`

Adjust `batch_size`, `parallel_requests`, and `synonyms_min`/`synonyms_max` to trade off speed vs. coverage for your model.

## How it works

### Index construction

`ctxt init` walks the filesystem tree and builds `.ctxt/ctx_index.json` — a single recursive JSON tree where every entry has `full_path`, `type`, optional `symbols`, optional `synonyms`, and nested `children` for directories.

**Symbols:** Every source file gets scanned with language-specific extractors that pull out exported symbols — functions, types, variables, classes, constants. These become a `symbols` array on the file node. The default extractor is `auto` (tree-sitter with regex fallback). Supported languages: Go (go/parser), Python/JavaScript/TypeScript/Rust/Svelte/Astro (tree-sitter), Vue/Ruby (regex fallback).

**Synonyms:** Each node (file or directory) gets 5–12 LLM-generated synonyms via batched API calls to an OpenRouter-compatible endpoint. For example, `Skeletons.tsx → ["skeletons", "loading", "placeholder", "animation"]`. Names are batched (default 15 per request, 10 parallel) and processed concurrently. The LLM prompt includes the file's extracted symbols (up to 10 per file) and, for JS/TS files, ESM imports to generate more contextual synonyms — conceptual terms, action verbs, and nouns that relate to the code's purpose. For example, a `route.ts` file with `import {clerkClient} from "@clerk/nextjs"` gets synonyms like "clerk webhook handler".

**Bootstrap diff:** On subsequent runs, `init` diffs the filesystem against the existing snapshot using file modification times. Deleted files are removed, new files are added, modified files are re-extracted. No LLM calls for existing synonyms — run `ctxt sync` to fill gaps. Synonym keys use `parentDir/basename` (e.g., `webhook/route.ts`) instead of bare basenames to prevent duplicate filenames from overwriting each other in the synonym map.

### Search mechanics

When you run `search-hints "product review rating"`, the query is lowercased and split into tokens. Each token is scored against every node in the tree using matchers that each contribute points:

| Matcher | Points | Description |
|---------|--------|-------------|
| exact | +12 | Token exactly matches a directory basename |
| basename | +7 | Token matches the file's basename |
| exact basename | +15 | Token exactly matches the full filename (with or without extension) |
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

Results are ranked by total score. Low-signal short/common words are filtered from query and synonym matching to reduce noise. Results are truncated at confidence gaps — if there's a 50%+ score drop between consecutive results, lower-scoring results are omitted even if `--limit` would include them.

### Watch mode

`ctxt watch` runs as a daemon, maintaining the index in memory. It watches for filesystem changes via debounced events (750ms default), re-extracts symbols for modified files, and serves a `.ctxt/ctx_runtime.json` that `search-hints --memory` reads for live results. The on-disk snapshot is flushed every 45s (`persist_interval`) and on graceful shutdown. This gives sub-second updates during active development.

## Commands

### `ctxt init`

Create a full snapshot in `.ctxt/ctx_index.json` with extracted symbols and optional synonyms.

```bash
ctxt init .
ctxt init . --output .ctxt/ctx_index.json --synonym-cache .ctxt/ctx_cache.json
```

Key flags:
- `--no-config-prompt`, `--create-config` — non-interactive automation
- `--llm-model`, `--batch-size`, `--synonyms-min`, `--synonyms-max`, `--api-key`, `--ignore`
- `--symbol-extractor` — symbol extraction mode: `auto` (default, tree-sitter with regex fallback), `treesitter`, or `regex`
- `-v, --verbose` — show symbol extraction progress and batch completion
- Always rebuilds the entire tree; use when you need a clean snapshot

On first run, creates a starter `.ctxt/ctx_config.toml` and pauses so you can edit it.

### `ctxt sync`

Targeted synonym generation — only generates synonyms for names that are missing or below `synonyms_min`. Works on the existing index without rebuilding.

```bash
ctxt sync .
```

Use after `init` to fill synonym gaps, or after adding new files to catch names that were missed.

### `ctxt watch`

Keep the index in memory with live filesystem updates.

```bash
ctxt watch . --debounce 750ms --verbose
```

Key flags:
- `--llm-on-watch` (default true) — live synonym enrichment for new files
- `--search-log` (default true) — log memory search requests
- `--search-log-query-max` (default 120) — truncate logged queries
- `--persist` (default "shutdown") — when to flush: `shutdown`, `interval`, `never`
- `--persist-interval` (default "45s") — flush interval when persist=interval
- Starts a local memory-search endpoint and writes `.ctxt/ctx_runtime.json`
- Events applied via a single worker; logs show changed files per cycle

### `ctxt search-hints`

Query the index for ranked paths with explainable scores.

```bash
ctxt search-hints "update storage" --json
ctxt search-hints "routing auth" --dir-summary --dir-limit 5 --drill-limit 3
```

Flags:
- `--limit`, `--min-score`, `--type files|dirs|all`
- `--dir-summary`, `--dir-limit`, `--drill-limit` — top-down directory-first results
- `--explain`, `--show-tokens`, `--json`
- `--memory` (default true) — query live watch index first, fall back to snapshot
- `--memory-only` — fail if live memory unavailable
- `--runtime-file` — path to runtime discovery file (default `.ctxt/ctx_runtime.json`)
- `--hybrid` — augment index results with content matching via ripgrep (default false)
- `--hybrid-score` — score assigned to content-matched results (default 1)
- `--hybrid-root` — project root for content matching (defaults to index root)

### `ctxt eval`

Benchmark Hit@1/3/5 + MRR from manual query cases.

```bash
ctxt eval --cases ctx_cases.json --json
```

Input format (v2 with categories, backward compatible with v1):
```json
{
  "version": 2,
  "categories": {
    "path-intent": {
      "description": "Find file by describing path/location purpose",
      "cases": [
        {"query": "auth middleware", "expect_any": ["internal/auth/middleware.go"]}
      ]
    }
  }
}
```

v1 format (bare array) is also supported:
```json
[
  {"query": "auth middleware", "expect_any": ["internal/auth/middleware.go"]}
]
```

### `ctxt bench`

Benchmark ctxt against find, grep, fd, rg, hybrid, and combined engines.

```bash
ctxt bench --cases docs/bench_cases.json --by-category
ctxt bench --cases docs/bench_cases.json --engines ctxt,find --json
```

Flags:
- `--cases` — path to case file (v2 format with categories)
- `--engines` — comma-separated list: ctxt,find,grep (default); also available: fd, rg, hybrid, combined
- `--by-category` — group results by category
- `--json` — output structured JSON
- `--limit` — max results per engine (default: 10)
- `--min-score` — minimum score threshold
- `--root` — project root
- `--index` — path to index file
- `--grep-max-bytes` — max bytes for grep content search

### `ctxt status`

Report index health, watch state, and path information.

```bash
ctxt status
ctxt status --json
```

### `ctxt clean`

Remove the `.ctxt/` directory.

```bash
ctxt clean
ctxt clean --dry-run
```

### `ctxt doctor`

Health-check config, root, index, cache, and API key.

```bash
ctxt doctor --json
```

### `ctxt config init`

Create or overwrite `.ctxt/ctx_config.toml`:

```bash
ctxt config init --output .ctxt/ctx_config.toml
```

## Configuration

`.ctxt/ctx_config.toml` drives all defaults. CLI flags override config, which overrides hard-coded defaults.

```toml
[common]
output = ".ctxt/ctx_index.json"
synonym_cache = ".ctxt/ctx_cache.json"
llm_model = "deepseek/deepseek-v4-flash"
batch_size = 15              # names per LLM request
synonyms_min = 5             # min synonyms per name
synonyms_max = 12            # max synonyms per name
ignore = [".git", ".venv", "site-packages", "__pycache__", "node_modules", "vendor", "dist", "migrations", "pb_migrations", "alembic", "flyway"]
dot_whitelist = []           # extra dot files to keep (merged with built-in defaults)
verbose = true

[llm]
provider = "openrouter"
endpoint = "https://openrouter.ai/api/v1/chat/completions"
model = "deepseek/deepseek-v4-flash"
api_key_env = "OPENROUTER_API_KEY"   # reads key from env var
temperature = 0.9
parallel_requests = 10       # concurrent LLM batches
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
init → walk filesystem → extract symbols → build symbols map → (LLM: generate synonyms WITH symbols) → .ctxt/ctx_index.json
                                                                                              ↓
watch → load .ctxt/ctx_index.json → keep in RAM → filesystem events → mutate in-memory
                                                                                              ↓
search-hints → load .ctxt/ctx_index.json (or query live memory) → score tokens → ranked results
```

## File formats

- **`.ctxt/ctx_index.json`** — root path, timestamp, tree with `full_path`, `type`, `symbols`, `synonyms`, `children`
- **`.ctxt/ctx_cache.json`** — basename → synonyms cache for reuse across runs
- **`.ctxt/ctx_config.toml`** — config-driven defaults
- **`.ctxt/ctx_runtime.json`** — live watch discovery for memory search
- **Bench/eval case files** — v2 format with categories (path-intent, symbol-lookup, concept-synonym, exact-file, narrow-scope, vague-intent); v1 bare array format is backward compatible

## Project size guard

Projects with more than ~9 synonym batches (>135 names at batch_size=15) trigger a warning. LLM reliability drops at scale. Use `ctxt sync` for targeted generation on large projects.

## Testing

```bash
go test ./...
```

## Troubleshooting

- `ctxt doctor --json` for diagnostics
- If `.ctxt/ctx_index.json` is stale, restart watch or run `ctxt init`
- If you changed ignore rules, run `ctxt init` or restart `watch` to rebuild
- Synonym generation requires `OPENROUTER_API_KEY` or `--api-key`. Disable with `--llm-on-watch=false` or `watch.llm = false`
- Watch mode must be stopped gracefully (Ctrl+C) to flush the snapshot
