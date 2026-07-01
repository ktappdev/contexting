# Benchmark Guide

## Overview

The `contexting bench` command compares how well different search engines find files in your codebase. It runs a set of test queries through multiple engines and reports which ones succeed, how fast they are, and how much noise they return.

This helps you understand:
- How contexting compares to traditional tools like `find` and `grep`
- Which search scenarios contexting excels at
- Where there might be room for improvement

## How to Run

### Basic benchmark

```bash
contexting bench --cases docs/bench_cases.json
```

This runs all 4 engines (contexting, find, grep, combined) against the case file and prints a summary.

### Run specific engines

```bash
contexting bench --cases docs/bench_cases.json --engines contexting,find
```

### JSON output

```bash
contexting bench --cases docs/bench_cases.json --json
```

### Group by category

```bash
contexting bench --cases docs/bench_cases.json --by-category
```

### Full example

```bash
contexting bench \
  --root . \
  --index .ctx/ctx_index.json \
  --cases docs/bench_cases.json \
  --engines contexting,find,grep,combined \
  --limit 10 \
  --by-category
```

## The 4 Engines

| Engine | What it does |
|--------|--------------|
| **contexting** | Uses the precomputed index with symbols, synonyms, and path scoring. Ranked results. |
| **find** | Walks the filesystem and matches tokens against filenames. Alphabetical results. |
| **grep** | Walks the filesystem and matches tokens against file contents. Alphabetical results. |
| **combined** | Union of find and grep results. Alphabetical results. |

**Key difference:** contexting returns ranked results (best match first). The other engines return alphabetical lists and don't rank by relevance.

## Metrics Explained

| Metric | What it means |
|--------|---------------|
| **Hit@1** | The correct file appeared as the first result |
| **Hit@3** | The correct file appeared in the top 3 results |
| **Hit@5** | The correct file appeared in the top 5 results |
| **Recall** | Percentage of cases where the engine found the correct file anywhere in results |
| **Avg Time** | Average time per query in milliseconds |
| **Avg Tokens** | Average total characters in results / 4 (heuristic for output size) |
| **Noise Ratio** | 1.0 - (relevant results / total results). Lower is better. 0.0 means no noise. |

**What to look for:**
- High Hit@1 means the engine usually puts the right file first
- High recall means the engine finds the right file, even if not first
- Low noise ratio means the engine doesn't return irrelevant files
- Fast time is good, but accuracy matters more for most use cases

## The 6 Categories

### path-intent

**What it tests:** Can the tool find a file when you describe what it does, not what it's named?

**Plain English:** You know what a file does, but you don't remember its name. You type a description hoping the tool can guess the file.

**Example query:** `"where is the search scoring logic"`

**Expected file:** `search.go`

**What a win looks like:** contexting returns `search.go` as the first result because it understands that "search scoring" maps to the file that contains the scoring logic.

---

### symbol-lookup

**What it tests:** Can the tool find which file contains a specific function or type?

**Plain English:** You know a function name (like `BuildIndex`) and want to know which file it lives in.

**Example query:** `"BuildIndex function"`

**Expected file:** `indexer.go`

**What a win looks like:** contexting returns `indexer.go` because it extracted the symbol `BuildIndex` during indexing and can match the query against it.

---

### concept-synonym

**What it tests:** Can the tool bridge the gap between your vocabulary and the code's vocabulary?

**Plain English:** You're thinking in different words than the code uses. You type "file watching" but the code says "fsnotify" and "monitor."

**Example query:** `"how to make the initial project setup"`

**Expected file:** `command_init.go`

**What a win looks like:** contexting returns `command_init.go` because LLM-generated synonyms linked "setup" to "init," even though the code never uses the word "setup."

---

### exact-file

**What it tests:** Baseline. Every tool should nail this.

**Plain English:** You know the exact filename and just want to find it.

**Example query:** `"commands.go"`

**Expected file:** `commands.go`

**What a win looks like:** All engines return `commands.go`. If contexting can't handle this, something is wrong.

---

### narrow-scope

**What it tests:** Can the tool filter out noise and give you exactly what you asked for?

**Plain English:** You want something specific (like tests) and don't want the source files cluttering your results.

**Example query:** `"eval report formatting not the eval scoring"`

**Expected file:** `eval_report.go`

**What a win looks like:** contexting returns `eval_report.go` and not `eval.go`. The tool understands the distinction between formatting and scoring.

---

### vague-intent

**What it tests:** Can the tool find things when you don't know any actual words from the code?

**Plain English:** You have a feeling or a general question, but no specific keywords that appear in filenames.

**Example query:** `"how does this tool know what files to skip"`

**Expected file:** `ignore.go`

**What a win looks like:** contexting returns `ignore.go` by connecting "what files to skip" to the concept of ignore logic, even though the query doesn't contain the word "ignore."

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
          "description": "Search scoring logic is in the file literally named search.go",
          "find_pattern": "search.go",
          "grep_pattern": "SearchHints|SearchResult"
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
- `find_pattern`: Pattern for the find engine to match
- `grep_pattern`: Pattern for the grep engine to match

## Adding Your Own Cases

1. Create a new JSON file following the v2 format above
2. Add categories and cases that reflect your project's structure
3. Run the benchmark with your file:

```bash
contexting bench --cases your_cases.json
```

**Tips for good cases:**
- Cover different scenarios: exact names, concepts, symbols, vague intent
- Use real queries you or agents would actually type
- Include both easy cases (exact filename) and hard cases (conceptual)
- Test edge cases specific to your project's naming conventions

## Interpreting Results

### Good results for contexting:
- **Hit@1 > 80%** in path-intent, symbol-lookup, and concept-synonym
- **Hit@1 = 100%** in exact-file (this should never fail)
- **Low noise ratio** (< 0.3) — means contexting isn't returning irrelevant files
- **Fast time** (< 50ms per query) — index lookup should be fast

### Contexting vs others:
- **contexting vs find:** contexting should win on conceptual queries; find may win on exact filename matches
- **contexting vs grep:** contexting should be faster and have less noise; grep may find more but with lower precision
- **contexting vs combined:** contexting should have higher Hit@1 due to ranking; combined may have higher recall but more noise

### When to investigate:
- If exact-file Hit@1 is below 100%, there's a bug
- If contexting is slower than find/grep, check index size or I/O
- If noise ratio is high, consider tuning min-score or improving synonym quality
- If symbol-lookup is weak, check that symbol extraction is working for your languages

## Related Commands

- `contexting init` — Build the index before running benchmarks
- `contexting search-hints --explain` — See why a query returns specific results
- `contexting doctor` — Check index and config health before benchmarking