# Contexting

**The command is `ctxt` — short for Contexting.**

Contexting keeps a live map of your codebase so AI agents can reason about paths without hunting through the filesystem manually. It builds a recursive JSON tree of every folder and file, extracts code symbols (functions, classes, types, variables) using language-specific extractors, attaches LLM-generated synonyms, and exposes ranked search hints plus health tooling.

Read the visual [getting started guide](https://ktappdev.github.io/contexting/) for the product story, architecture, installation paths, MCP setup, and privacy notes.

## Quick start

`ctxt` supports Linux, macOS, and Windows on amd64 and arm64. Download the
archive for your platform from GitHub Releases, verify it against
`checksums.txt`, and place the binary on your `PATH`. Go users can install
from source:

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
- **Other OpenAI-compatible APIs** — set `endpoint`, `model`, and `api_key_env`
- **No LLM** — skip the `[llm]` section entirely or set `api_key = ""`

Adjust `batch_size`, `parallel_requests`, and `synonyms_min`/`synonyms_max` to trade off speed vs. coverage for your model.
Remote LLM endpoints must use HTTPS; HTTP is accepted only on loopback.

## How it works

### Index construction

`ctxt init` walks the filesystem tree and builds `.ctxt/ctx_index.json` — a single recursive JSON tree where every entry has `full_path`, `type`, optional `symbols`, optional `synonyms`, and nested `children` for directories.

**Symbols:** Every source file gets scanned with language-specific extractors that pull out exported symbols — functions, types, variables, classes, constants. These become a `symbols` array on the file node. The default extractor is `auto` (tree-sitter with regex fallback). Supported languages: Go (go/parser), Python/JavaScript/TypeScript/Rust/Svelte/Astro (tree-sitter), Vue/Ruby (regex fallback).

**Synonyms:** Each node (file or directory) gets 5–12 LLM-generated synonyms via batched API calls to an OpenAI-compatible endpoint. For example, `Skeletons.tsx → ["skeletons", "loading", "placeholder", "animation"]`. Names are smart-batched (up to 60 per request, 10 parallel by default). The prompt includes extracted symbols and, for JS/TS files, ESM imports to generate contextual terms. Use `--offline` to guarantee that no LLM requests are made.

**Bootstrap diff:** On subsequent runs, `watch` diffs the filesystem against the existing snapshot using file modification times and sizes. Timestamp-preserving edits with a different size are detected; edits preserving both size and timestamp require a full rebuild. Deleted files are removed, new files are added, modified files are re-extracted. Watch bootstrap reuses cached synonyms; run `ctxt sync` to fill gaps and refresh changed symbol/import context. Synonym cache keys use the full project-relative path (e.g., `billing/routes/index.ts`) so identically named files in separate features remain distinct.

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

Results are ranked by total score. Low-signal short/common words are filtered from query and synonym matching to reduce noise. Results are truncated at confidence gaps — if there's a 50%+ score drop between consecutive results, lower-scoring results are omitted even if `--limit` would include them. Exact filename/symbol lookups can return just one strong match; conceptual queries retain up to three alternatives. Zero-score candidates are never returned.

### Watch mode

`ctxt watch` maintains the index in memory. It watches for filesystem changes via debounced events (750ms default), re-extracts symbols for modified files, and serves a loopback endpoint advertised by `.ctxt/ctx_runtime.json`. The on-disk snapshot is flushed on graceful shutdown.

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

Targeted synonym generation — generates synonyms for indexed names that are missing, below `synonyms_min`, or have changed symbols/imports. It refreshes symbol extraction from disk before comparing context fingerprints. Works on the existing index without rebuilding the file tree.

```bash
ctxt sync .
ctxt sync . --path command_sync.go
ctxt sync . --path docs --force
```

`--path` selects an indexed file or directory; `--force` regenerates selected
entries even if their fingerprints are current. Use `--force` after changing
models or generation instructions. Body-only edits that leave symbols/imports
unchanged also require `--force`.

Successful LLM responses are saved even if another batch fails. The command
still exits with an error; rerun without `--force` to request only missing,
short, or stale entries. `--batch-size` explicitly controls batch size when set.

Only version 2 caches and indexes are supported. Older files are rejected;
there is no migration or synonym-key fallback. For the default paths, rebuild:

```bash
rm .ctxt/ctx_cache.json
ctxt init .
```

Use `ctxt init . --offline` to rebuild without LLM requests.

Use `init` or restart `watch` after adding files, then run `sync` to fill gaps.

### `ctxt watch`

Keep the index in memory with live filesystem updates.

```bash
ctxt watch . --debounce 750ms --verbose
```

Key flags:
- `--llm-on-watch` (default false) — opt into live remote synonym enrichment
- `--search-log` (default false) — log memory search requests; queries may be sensitive
- `--search-log-query-max` (default 120) — truncate logged queries
- `--persist` (default "shutdown") — snapshots flush on graceful shutdown
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
- `--hybrid` — augment index results by searching indexed files with ripgrep (default false); honors type and minimum-score filters
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
batch_size = 8               # names per LLM request; 0 enables smart batching
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

**Supported providers:** OpenRouter (default) and APIs that implement the
OpenAI chat-completions request/response shape. The `provider` value labels
the configuration; `endpoint` and `model` control the request.

## Ignore system

Contexting loads common `.gitignore` patterns by default. Additional ignores come from:

1. **Built-in defaults** — `.git`, `.venv`, `site-packages`, `__pycache__`, `node_modules`, `vendor`, `dist`, `migrations`, `pb_migrations`, `alembic`, `flyway`
2. **`.gitignore` patterns** — loaded from the project root
3. **`ignore` in config** — extra patterns merged with defaults

Root `.gitignore` rules support ordered negation (`!path`), rooted patterns,
directory-only patterns, and `**` globs. Negation can undo earlier Git rules,
but cannot re-include an excluded parent directory or override built-in/config
exclusions. Symlink entries are not followed or indexed.

**Dot files** are skipped by default (any path segment starting with `.`). whitelisted dot files (`.env.example`, `.prettierrc`, `.editorconfig`, etc.) remain eligible unless explicitly ignored. Add more via `dot_whitelist` in config.

## Data flow

```
init → walk filesystem → extract symbols → build symbols map → (LLM: generate synonyms WITH symbols) → .ctxt/ctx_index.json
                                                                                              ↓
watch → load .ctxt/ctx_index.json → keep in RAM → filesystem events → mutate in-memory
                                                                                              ↓
search-hints → load .ctxt/ctx_index.json (or query live memory) → score tokens → ranked results
```

## File formats

- **`.ctxt/ctx_index.json`** — schema version, root path, timestamp, and indexed tree
- **`.ctxt/ctx_cache.json`** — versioned project-relative synonyms and symbol/import context fingerprints
- **`.ctxt/ctx_config.toml`** — config-driven defaults
- **`.ctxt/ctx_runtime.json`** — live watch discovery for memory search
- **Bench/eval case files** — v2 format with categories (path-intent, symbol-lookup, concept-synonym, exact-file, narrow-scope, vague-intent); v1 bare array format is backward compatible

## Project size guard

Initial indexing supports up to 10,000 files after ignores. Larger projects
fail with an actionable error instead of producing a partial index. Add ignore
patterns and retry. At 5,000 files, ctxt warns that narrower ignores may help.

## Testing

```bash
go test ./...
```

## Troubleshooting

- `ctxt doctor --json` for diagnostics
- If `.ctxt/ctx_index.json` is stale, restart watch or run `ctxt init`
- If you changed ignore rules, run `ctxt init` or restart `watch` to rebuild
- Synonym generation requires the configured key environment or `--api-key`.
  Use `--offline` to disable every LLM request regardless of configuration.
- Watch mode must be stopped gracefully (Ctrl+C) to flush the snapshot
- If a writer crashes, verify no ctxt writer remains before removing the
  empty `.ctxt-writer` directory.
- See [SECURITY.md](SECURITY.md) before enabling a remote LLM on private code.
