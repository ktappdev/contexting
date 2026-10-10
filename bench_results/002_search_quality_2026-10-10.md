# Search quality check — 2026-10-10

Compared the existing confidence-gap behavior with a narrower change: exact
filename or exact symbol matches may stop at the first score cliff without
padding to three results. Conceptual queries retain the existing three-result
minimum when enough candidates exist. Candidates with no matching signal are
excluded even when the caller sets `MinScore` to zero.

## Method

- Existing `docs/bench_cases.json`: 45 queries across six categories.
- Same saved index for both runs, built from this repository with local symbol
  extraction and lexical synonyms. No LLM requests or paid services.
- `ctxt` engine only, limit 10, minimum score 1, category reports enabled.
- This checks the cutoff change; it does not measure LLM synonym quality or
  reproduce the separate monorepo used in `docs/contexting_feedback.md`.

## Results

| Metric | Before | After |
| --- | ---: | ---: |
| Hit@1 | 64.44% | 64.44% |
| Hit@3 | 82.22% | 82.22% |
| Hit@5 | 86.67% | 86.67% |
| Recall | 86.67% | 86.67% |
| Average results | 7.69 | 6.27 |
| Average estimated tokens | 32.60 | 27.31 |
| Average noise ratio | 80.69% | 67.85% |

Hit rates and recall were unchanged in every category. Exact filename queries
already had 100% Hit@1. Vague intent remains the weakest category, with 14.29%
Hit@1 on this offline index; these fixes do not resolve that vocabulary gap.

Applying the stricter cutoff to every query reduced recall to 84.44%, so that
broader change was rejected.

## Validation

The existing `go test ./...` suite and `go vet ./...` passed. Temporary CLI
fixtures also confirmed ordered Git negation, rooted/directory/recursive
patterns, additive config exclusions, whitelisted dot files, symlink exclusion,
newline filenames, hybrid type/minimum-score filtering, and watch bootstrap
refreshing symbols after an edit with preserved modification time but new size.
