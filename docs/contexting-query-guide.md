# Contexting Query Guide: Getting the Best Results from `search-hints`

## Basic Usage

```bash
contexting search-hints "your query here"
```

The query is a **positional argument** (not a flag). It returns the top 10 matching paths by default, ranked by relevance score (0–100+).

## Tips for Better Queries

### 1. Use natural language, not code

Contexting indexes code *symbols* (function names, types, variables) and LLM-generated synonyms. So you can describe what the code *does* rather than guessing filenames:

```bash
# Good — describes purpose
contexting search-hints "spawns cli subprocess for batch processing"

# Also good — topic-focused
contexting search-hints "profanity detection and audio cleaning"
```

### 2. Be specific but not verbose

Contexting normalizes query tokens and matches against symbols and synonyms. A few well-chosen keywords work better than a single vague word or a full paragraph:

```bash
# Too vague
contexting search-hints "code"

# Too long
contexting search-hints "how does the desktop application install the command line tool for batch audio file processing"

# Sweet spot
contexting search-hints "cli install and batch processing"
```

### 3. Use `--show-tokens` to see how your query is parsed

If results seem off, check what tokens contexting extracted:

```bash
contexting search-hints "batch audio" --show-tokens
# Tokens: [batch audio]
```

This helps you understand why certain files matched (or didn't). Each result shows its match sources (`basename`, `path`, `syn`, `sym`).

### 4. Use multiple queries from different angles

No single query covers everything. Try different perspectives to find all relevant files:

```bash
contexting search-hints "cli install"
contexting search-hints "batch audio processing"
contexting search-hints "api key and authentication"
```

## Display Modes

### Standard (default)

Shows file paths with scores:

```bash
contexting search-hints "transcribe"
# badwords-editor-solid/frontend/src/store/transcribe.ts       (file) score=100
# lyricut-cli/transcribe.go                                     (file) score=45
```

### Summary (`--summary`)

Compact output — path, type, score only. Best for piping or quick scanning:

```bash
contexting search-hints "transcribe" --summary
```

### Directory Summary (`--dir-summary`)

Groups results by directory with rationale — ideal when you're unfamiliar with the project layout:

```bash
contexting search-hints "batch audio" --dir-summary
# 1. badwords-editor-solid/cli (score=232, hits=2)
#    rationale: matched BatchProgress, CLICleanOptions, batch
#    - cli_install.go (131)
#    - cli_batch.go (101)
```

Flags `--dir-limit` (default 5) and `--drill-limit` (default 3) control how many directories and hits-per-directory are shown.

### Explain (`--explain`)

Shows score breakdown — useful for debugging why certain files ranked higher than expected:

```bash
contexting search-hints "profanity detection" --explain --limit 3 --summary
```

### JSON (`--json`)

Machine-readable output for scripting or tooling:

```bash
contexting search-hints "batch processing" --json
```

## Filtering

### By type

Limit results to files or directories:

```bash
contexting search-hints "batch" --type files
contexting search-hints "batch" --type dirs
```

### By score threshold

Exclude low-confidence matches:

```bash
contexting search-hints "batch" --min-score 30
```

### By result count

```bash
contexting search-hints "transcribe" --limit 3
```

## Live Memory Query (Fast Mode)

When `contexting watch` is running (it keeps the index in memory), `search-hints` queries it automatically via `--memory` (default true). This is ~20x faster than filesystem `find` commands:

```bash
# Results in ~8ms vs ~200ms for find
contexting search-hints "cli install" --limit 1
```

Use `--memory-only` to fail if the watch daemon isn't running (ensures you're getting the fastest path).

## Choosing the Right Index

By default, `search-hints` uses `.ctx/ctx_index.json` in the current directory. Point it at a different project's index:

```bash
contexting search-hints "oauth" --index /path/to/other-project/.ctx/ctx_index.json
```

Or set the root to search relative to a different project root:

```bash
contexting search-hints "transcribe" --root /path/to/other-project
```

## Pitfalls to Avoid

| Mistake | Why |
|---|---|
| Using `--query` flag | `search-hints` takes the query as a positional arg, not a flag |
| Single generic word like "handler" or "config" | Too many matches, low scores. Be more specific |
| Expecting full-text search | Contexting indexes *symbols*, not every line of code. It won't find variable values or comments |
| No index | `contexting init` or `contexting watch` must have been run first. `contexting status` to check |
| Forgetting `--memory` | If the watch daemon died, it falls back to the snapshot file. Restart with `contexting watch` |
