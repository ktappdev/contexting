# Benchmark Guide

## Overview

The `ctxt bench` command compares how well different search engines find files in your codebase. It runs a set of test queries through multiple engines and reports which ones succeed, how fast they are, and how much noise they return.

This helps you understand:
- How ctxt compares to traditional tools like `find` and `grep`
- Which search scenarios ctxt excels at
- Where there might be room for improvement

## How to Run

### Basic benchmark

```bash
ctxt bench --cases docs/bench_cases.json
```

This runs the default engines (ctxt, find, grep), with one warmup and five measured runs per query. Every engine returns at most 10 files from the same indexed file set. Expected paths must identify exact project-relative files, including case.

### Run specific engines

```bash
ctxt bench --cases docs/bench_cases.json --engines ctxt,find
```

### JSON output

```bash
ctxt bench --cases docs/bench_cases.json --json
```

### Group by category

```bash
ctxt bench --cases docs/bench_cases.json --by-category
```

### Full example

```bash
ctxt bench \
  --root . \
  --index .ctxt/ctx_index.json \
  --cases docs/bench_cases.json \
  --engines ctxt,find,grep,fd,rg,hybrid,combined \
  --limit 10 \
  --by-category
```

## Bench Engines

| Engine | What it does |
|--------|--------------|
| **ctxt** | Uses the precomputed index with symbols, synonyms, and path scoring. Ranked results. |
| **find** | Executes find, intersects its file list with the index, and filters by query tokens. Alphabetical results. |
| **fd** | Executes fd with native ignore filtering disabled, then applies the same indexed file set and token filter as find. Alphabetical results. |
| **grep** | Executes grep with fixed-string, case-insensitive patterns against explicit indexed files. Alphabetical results. |
| **rg** | Executes ripgrep with the same fixed-string patterns and explicit indexed files. Alphabetical results. |
| **hybrid** | ctxt index + ripgrep content fallback for sparse results. Content matches at score=1. |
| **combined** | Union of find and grep results. Alphabetical results. |

**Key difference:** ctxt returns ranked results (best match first). The other engines return alphabetical lists and don't rank by relevance. hybrid combines ctxt ranking with content fallback.

## Metrics Explained

| Metric | What it means |
|--------|---------------|
| **Hit@1** | The correct file appeared as the first result |
| **Hit@3** | The correct file appeared in the top 3 results |
| **Hit@5** | The correct file appeared in the top 5 results |
| **Success** | Percentage of queries with any acceptable expected file within the shared result limit; this is not conventional document recall |
| **Avg Time / p50 / p95** | Measurements pooled across repeated adapter calls, in fractional milliseconds; warmups and failed cases are excluded |
| **Estimated path tokens** | Sum of returned path bytes / 4; excludes JSON, metadata, separators, and file contents |
| **Noise Ratio** | 1.0 - (labeled acceptable results / total returned results). Unlabeled files count as noise, so labels must include all acceptable answers. Empty results have zero noise but also zero success. |
| **Errors** | Cases where an executable is missing or execution fails. These cause a nonzero CLI exit and are separate from search misses. |

**What to look for:**
- High Hit@1 means the engine usually puts the right file first
- High success means an acceptable file appears within the same result budget
- Low noise ratio means the engine doesn't return irrelevant files
- Fast time is good, but accuracy matters more for most use cases

## The 6 Categories

### path-intent

**What it tests:** Can the tool find a file when you describe what it does, not what it's named?

**Plain English:** You know what a file does, but you don't remember its name. You type a description hoping the tool can guess the file.

**Example query:** `"where is the search scoring logic"`

**Expected file:** `search.go`

**What a win looks like:** ctxt returns `search.go` as the first result because it understands that "search scoring" maps to the file that contains the scoring logic.

---

### symbol-lookup

**What it tests:** Can the tool find which file contains a specific function or type?

**Plain English:** You know a function name (like `BuildIndex`) and want to know which file it lives in.

**Example query:** `"BuildIndex function"`

**Expected file:** `indexer.go`

**What a win looks like:** ctxt returns `indexer.go` because it extracted the symbol `BuildIndex` during indexing and can match the query against it.

---

### concept-synonym

**What it tests:** Can the tool bridge the gap between your vocabulary and the code's vocabulary?

**Plain English:** You're thinking in different words than the code uses. You type "file watching" but the code says "fsnotify" and "monitor."

**Example query:** `"how to make the initial project setup"`

**Expected file:** `command_init.go`

**What a win looks like:** ctxt returns `command_init.go` because LLM-generated synonyms linked "setup" to "init," even though the code never uses the word "setup."

---

### exact-file

**What it tests:** Baseline. Every tool should nail this.

**Plain English:** You know the exact filename and just want to find it.

**Example query:** `"commands.go"`

**Expected file:** `commands.go`

**What a win looks like:** All engines return `commands.go`. If ctxt can't handle this, something is wrong.

---

### narrow-scope

**What it tests:** Can the tool filter out noise and give you exactly what you asked for?

**Plain English:** You want something specific (like tests) and don't want the source files cluttering your results.

**Example query:** `"eval report formatting not the eval scoring"`

**Expected file:** `eval_report.go`

**What a win looks like:** ctxt returns `eval_report.go` and not `eval.go`. The tool understands the distinction between formatting and scoring.

---

### vague-intent

**What it tests:** Can the tool find things when you don't know any actual words from the code?

**Plain English:** You have a feeling or a general question, but no specific keywords that appear in filenames.

**Example query:** `"how does this tool know what files to skip"`

**Expected file:** `ignore.go`

**What a win looks like:** ctxt returns `ignore.go` by connecting "what files to skip" to the concept of ignore logic, even though the query doesn't contain the word "ignore."

## Case File Format

The case file (`docs/bench_cases.json`) uses a v2 format with categories:

```json
{
  "version": 2,
  "categories": {
    "path-intent": {
      "description": "Find file by describing path/location purpose. Tests fuzzy path matching.",
      "cases": [
        {
          "query": "where is the search scoring logic",
          "expect_any": ["search.go"],
          "description": "Search scoring logic is in search.go"
        }
      ]
    }
  }
}
```

**Fields per case:**
- `query`: The search query to run
- `expect_any`: Array of acceptable file paths (any match counts as success)
- `description`: Human-readable explanation of what the case tests
Every engine receives the same query. There are no engine-specific query hints. Expected paths are checked against the indexed file list before execution.

## Adding Your Own Cases

1. Create a JSON file outside the indexed corpus (for example `.ctxt/validation_cases.json`) following the v2 format above
2. Add categories and cases that reflect your project's structure
3. Run the benchmark with your file:

```bash
ctxt bench --cases your_cases.json
```

**Tips for good cases:**
- Cover different scenarios: exact names, concepts, symbols, vague intent
- Use real queries you or agents would actually type
- Include both easy cases (exact filename) and hard cases (conceptual)
- Include similar suffixes, duplicate basenames, and case-sensitive paths. List every acceptable file explicitly.

## Interpreting Results

### Good results for ctxt:
- **Hit@1 > 80%** in path-intent, symbol-lookup, and concept-synonym
- **Hit@1 = 100%** in exact-file (this should never fail)
- **Low noise ratio** (< 0.3) — means ctxt isn't returning irrelevant files
- **Fast time** (< 50ms per query) — index lookup should be fast

### Comparing engines fairly

All engines search files, excluding directory results. The saved index defines
membership and ignore rules for every engine; content engines additionally share
`--grep-max-bytes` (default 1 MiB). grep/rg match any literal query token,
case-insensitively. Baseline output is alphabetical, so Hit@K describes output
position rather than relevance ranking. Combined unions full find/grep candidates
before applying the result limit.

Use `--runs 10 --warmup 2` for more timing samples. Engine order rotates across
queries and repetitions. Index loading and corpus preparation are reported
separately. ctxt runs in process; find/fd/grep/rg include subprocess startup.
These are adapter timings, not complete CLI, watch HTTP, or MCP latency. They do
not include index construction or LLM generation cost. Use a stable checkout and
rebuild an outdated index before comparing runs.

JSON uses report version 2 with one structure for grouped and ungrouped runs:
`cases`, per-case `results` (including timing samples and errors), `summary`,
optional `categories`, `methodology`, and setup timings. Metric names include
`success_rate`, `estimated_path_tokens`, and `avg_estimated_path_tokens`.
Each result’s `time_ms` is the median of its measured `samples_ms`.
Previous report field names and suffix-based matching are not supported.

### Development and validation sets

Use `docs/bench_cases.json` while developing ranking changes. The separate
`docs/bench_validation_cases.json` contains 12 additional frozen questions. Run
it after choosing a change; do not tune scoring against its results. It still
uses this repository and cannot establish performance on other projects.
For claims beyond Contexting, collect queries and all acceptable paths from
other real repositories, freeze those revisions, and evaluate separately.

```bash
ctxt bench --cases docs/bench_validation_cases.json --engines ctxt,rg,hybrid --json
```

### Symbol extraction mode
Bench results depend on the symbol extraction mode used when building the index. To compare tree-sitter vs regex:

```bash
# Re-index with tree-sitter
ctxt init . --symbol-extractor treesitter
ctxt bench --cases docs/bench_cases.json --engines ctxt

# Re-index with regex
ctxt init . --symbol-extractor regex
ctxt bench --cases docs/bench_cases.json --engines ctxt
```

### When to investigate:
- If exact-file Hit@1 is below 100%, there's a bug
- If ctxt is slower than find/grep, check index size or I/O
- If noise ratio is high, consider tuning min-score or improving synonym quality
- If symbol-lookup is weak, check that symbol extraction is working for your languages

## Related Commands

- `ctxt init` — Build the index before running benchmarks
- `ctxt search-hints --explain` — See why a query returns specific results
- `ctxt doctor` — Check index and config health before benchmarking