# Benchmark Reports

## Purpose

This directory contains benchmark reports tracking contexting's search performance over time. Each report captures a point-in-time snapshot so you can compare before/after changes to scoring algorithms, synonym generation, symbol extraction, or other search-related features.

For user-facing documentation on how to run benchmarks and what metrics mean, see [`docs/bench_guide.md`](../docs/bench_guide.md).

## File Naming Convention

- **Format:** `NNN_bench_report_YYYY-MM-DD_HHMM.md`
- **NNN:** 3-digit incrementing number (001, 002, 003...)
- **YYYY-MM-DD:** Date of the bench run
- **HHMM:** Time of the bench run (UTC, 24-hour format)
- **Example:** `001_bench_report_2026-07-01_0310.md`

Always zero-pad numbers and time components. To determine the next number, list existing files in this directory and increment the highest number.

## When to Create a Report

Create a new benchmark report after any change that could affect search quality:

- Changes to scoring weights or algorithms
- Modifications to synonym generation (prompts, model, temperature)
- Updates to symbol extraction logic
- Changes to tokenization or query processing
- Reindexing with a new model or config
- Adding or modifying bench cases in `docs/bench_cases.json`
- When you want to track progress over time (e.g., before/after a feature)

## Required Sections

### 1. Header

```markdown
# Benchmark Report NNN — YYYY-MM-DD HH:MM UTC
```

Use the incrementing number and the actual date/time of the bench run.

### 2. Summary

Bullet points covering:
- Date/time of run
- Index info: model used, generated_at timestamp (if available)
- Cases: total count, number of categories
- Engines tested

Example:
```markdown
- **Date/time of run:** 2026-07-01 03:10 UTC
- **Index info:** Model: deepseek/deepseek-v4-flash
- **Cases:** 45 total across 6 categories
- **Engines tested:** contexting, find, grep, combined
```

### 3. Overall Results

A table with all engines and all metrics:

| Engine    | Hit@1  | Hit@3  | Hit@5  | Avg Time | Avg Tokens | Noise |
|-----------|--------|--------|--------|----------|------------|-------|
| contexting| 0.60   | 0.80   | 0.82   | 0ms      | 37         | 0.80  |
| find      | 0.24   | 0.44   | 0.51   | 0ms      | 27         | 0.57  |
| grep      | 0.02   | 0.13   | 0.16   | 2ms      | 182        | 0.95  |
| combined  | 0.02   | 0.13   | 0.18   | 2ms      | 186        | 0.94  |

Followed by 2-3 bullet points of key findings from the overall results.

### 4. Category Breakdown

One subsection per category (6 total). Each with:

- Category name and case count
- Results table for all engines
- 1-2 sentence analysis of what the results mean

Example:
```markdown
### narrow-scope (7 cases)

| Engine    | Hit@1  | Hit@3  | Hit@5  | Avg Time | Avg Tokens | Noise |
|-----------|--------|--------|--------|----------|------------|-------|
| contexting| 0.86   | 1.00   | 1.00   | 0ms      | 42         | 0.89  |
| find      | 0.00   | 0.29   | 0.43   | 0ms      | 33         | 0.75  |
| grep      | 0.00   | 0.29   | 0.29   | 2ms      | 206        | 0.96  |
| combined  | 0.00   | 0.29   | 0.29   | 2ms      | 208        | 0.96  |

**Analysis:** Contexting dominates this category with 86% Hit@1, demonstrating strong precision in filtering noise. Find and grep both achieve 0% Hit@1 — they return results but can't distinguish between similar files.
```

### 5. Per-Query Highlights

Three subsections:

**Contexting Hit@1 Wins:** List queries where contexting placed the correct file first. Group by category. Include query string, expected file, and rank.

**Contexting Losses or Misses:** List queries where contexting didn't achieve Hit@1. Include query string, expected file, what happened (rank, or "Found: no" for complete misses).

**Find/Grep Wins:** List queries where find or grep achieved Hit@1 but contexting didn't. If none, state that explicitly.

### 6. Key Findings

Numbered list of the most important takeaways. Include:

- Which categories contexting wins/loses
- Performance comparisons between engines
- Notable strengths or weaknesses
- Any bugs fixed or changes made since the last report

If a previous report exists, include comparisons (improvements, regressions).

### 7. Areas for Improvement

Subsections for the weakest categories with:
- Specific Hit@1 numbers
- Specific failing queries (especially "Found: no" cases)
- Suggested next steps or investigations

Focus on actionable insights — what should be changed to improve performance.

### 8. Configuration

List the configuration used for this benchmark:

- Model used for synonym generation
- Temperature
- Synonym range (min-max per name)
- Parallel requests
- Index generated_at timestamp (if available)
- Cases file used (path, case count, format version)

### 9. Methodology

Brief coverage of:

- Engines tested and how they work (contexting vs find vs grep vs combined)
- Metrics definitions (reference [`docs/bench_guide.md`](../docs/bench_guide.md) for full definitions)
- Any bugs fixed or changes made since the last report
- Test environment notes (index load time, sequential vs parallel, etc.)

## Comparison with Previous Reports

When a previous report exists, add a "Comparison" section after "Key Findings" showing deltas:

```markdown
## Comparison to Report 001

| Metric        | Before | After | Delta |
|---------------|--------|-------|-------|
| Overall Hit@1 | 0.60   | 0.64  | +0.04 |
| Vague-intent  | 0.14   | 0.29  | +0.15 |
| Noise ratio   | 0.80   | 0.75  | -0.05 |
```

- Note any regressions explicitly
- Highlight improvements
- Explain what changed between reports (code changes, config changes, new cases)

## How to Generate a Report

1. **Run the benchmark and capture output:**
   ```bash
   contexting bench --cases docs/bench_cases.json --no-config-prompt > /tmp/bench_text.txt
   ```

2. **Run with JSON for structured data:**
   ```bash
   contexting bench --cases docs/bench_cases.json --no-config-prompt --json > /tmp/bench_json.txt
   ```

3. **Determine the next incrementing number:**
   ```bash
   ls bench_results/
   # If highest is 001, next is 002
   ```

4. **Create the file with proper naming:**
   ```bash
   # Format: NNN_bench_report_YYYY-MM-DD_HHMM.md
   # Example: 002_bench_report_2026-07-02_1430.md
   ```

5. **Fill in all required sections** using real data from the bench output. Never estimate or fabricate numbers.

6. **If a previous report exists**, add a Comparison section with deltas.

## Tips

- **Use real numbers only** — extract exact values from the JSON output or text output
- **Include specific query strings** when discussing wins/losses — don't summarize
- **Note code changes** made since the last report (bug fixes, prompt changes, config changes)
- **Keep it readable** — use tables for data, prose for analysis
- **Be specific about failures** — list the exact queries where contexting missed (Found: no)
- **Reference the bench guide** for metric definitions rather than duplicating them

## Related Documentation

- [`docs/bench_guide.md`](../docs/bench_guide.md) — How to run benchmarks, metric definitions, category explanations
- [`docs/bench_cases.json`](../docs/bench_cases.json) — The test case file (45 cases across 6 categories)
