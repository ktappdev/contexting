# Contexting

Contexting builds a project index (`context.json`) and optional synonym layer so agents can map natural-language requests to the right files quickly.

## Why this is useful

- `init`: fast project snapshot with path + synonym hints
- `watch`: keeps index continuously fresh while coding
- `search-hints`: deterministic ranked path retrieval for agent prompts/tools
- `eval`: quality metrics (`Hit@k`, `MRR`) so ranking improvements are measurable
- `doctor`: one-shot health checks with fixes for config/index/cache/env setup

## Requirements

- Go 1.22+
- OpenRouter API key (optional, only needed for LLM synonym generation)

## Setup

```bash
go mod tidy
export OPENROUTER_API_KEY="sk-or-v1-..."
cp context.toml.example context.toml
```

All commands accept `--config context.toml` (default: `context.toml`).
Precedence is: CLI flags > `context.toml` > built-in defaults.
Relative paths in `init/watch` outputs are resolved from the target project root.
Relative `search/eval` paths from config are resolved from the config file directory.

On first run, if the config file is missing, Contexting prompts to create a starter config.
Useful global flags:

- `--create-config` auto-create starter config when missing (good for scripts/CI)
- `--no-config-prompt` disable the interactive prompt

You can also create config explicitly:

```bash
go run . config init
```

## Commands

### Build index

```bash
go run . init . --output context.json
```

Useful flags:

- `--llm-model openrouter/free`
- `--batch-size 8`
- `--synonyms 4`
- `--synonym-cache .contexting_synonyms_cache.json`
- `--api-key ...`
- `--ignore dist --ignore build`

If LLM calls fail, indexing still succeeds and adds lexical synonyms from identifiers (camelCase/snake_case/kebab-case splits).
Contexting also reads ignore rules from the project `.gitignore`; if `.gitignore` is missing, it creates a starter one with common defaults.

### Watch mode

```bash
go run . watch . --output context.json --debounce 750ms
```

Runs initial index, watches recursively, debounces event bursts, and rewrites both context index and synonym cache.
By default, watch mode does not call the LLM (fast startup/responsive events). Enable with `--llm-on-watch` or `watch.llm = true` in config.

### Search hints

```bash
go run . search-hints "check local storage" --index context.json -n 5 --explain
```

Useful flags:

- `--type all|files|dirs`
- `--min-score 1`
- `--json`
- `--show-tokens`
- `--explain` (score breakdown per result)

### Evaluate quality

```bash
go run . eval --index context.json --cases eval_cases.json -n 5
```

`eval_cases.json` format:

```json
[
  {"query": "check local storage", "expect_any": ["config/local_store.go", "config/store.go"]},
  {"query": "auth middleware", "expect_any": ["internal/auth/middleware.go"]}
]
```

Outputs `Hit@1`, `Hit@3`, `Hit@5`, and `MRR`, plus misses.

### Health checks

```bash
go run . doctor
```

Useful flags:

- `--json` machine-readable report
- `--root` project root override
- `--index` index path override
- `--synonym-cache` cache path override
- `--write-check=false` skip temp write-permission check

## JSON shape

`context.json` contains:

- `root_path`
- `generated_at`
- `model`
- `tree` recursively with:
  - `full_path`
  - `type` (`file`/`directory`)
  - `synonyms`
  - `children`

## Notes

- Writes are atomic (`context.json` and cache), so readers should not see partial files.
- Synonyms are filtered and capped to reduce noise.
- Ranking is lexical/heuristic (deterministic), not embedding-based.

## Test

```bash
go test ./...
```
# contexting
