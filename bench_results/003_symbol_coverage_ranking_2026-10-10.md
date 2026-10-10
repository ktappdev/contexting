# Symbol coverage and bounded ranking — 2026-10-10

## Method

Compared commit b9a3942 with the new extractor and scoring changes, using the
same archived repository source and the existing 45 cases in
`docs/bench_cases.json`. The baseline index used the previous extractor; the
final index was rebuilt with the new extractor. Both used offline lexical
synonyms, the ctxt engine, limit 10, and minimum score 1. No LLM calls were made.
These cases informed scoring changes, so this is a regression check rather than
an independent evaluation on unseen projects.

## Changes

- Include Go receiver methods alongside package declarations.
- Include indented Python methods and named default JS/TS exports in regex mode.
- Retain camelCase boundaries before splitting symbol names.
- Award each token its strongest path match and strongest synonym match.
- Cap symbol evidence at three distinct matches per token; exact symbol names
  receive +24 plus a +4 token bonus, so they outweigh three partial symbol matches (+9 each at most).
- Reward additional distinct query terms with +4 each; generated stems do not
  count as extra terms.

## Results

| Metric | Before | After |
| --- | ---: | ---: |
| Hit@1 | 66.67% | 71.11% |
| Hit@3 | 82.22% | 84.44% |
| Hit@5 | 86.67% | 86.67% |
| Recall at limit 10 | 86.67% | 88.89% |
| Average noise ratio | 69.89% | 68.74% |
| Average results | 6.44 | 6.40 |
| Average estimated tokens | 28.51 | 25.62 |

| Category | Hit@1 before | Hit@1 after |
| --- | ---: | ---: |
| concept-synonym | 50.00% | 37.50% |
| exact-file | 100.00% | 100.00% |
| narrow-scope | 57.14% | 85.71% |
| path-intent | 87.50% | 87.50% |
| symbol-lookup | 85.71% | 100.00% |
| vague-intent | 14.29% | 14.29% |

The correct file ranks first in 32/45 cases (previously 30), appears in the top
three in 38/45 (previously 37), and appears within the limit in 40/45 (previously
39). Symbol lookups improve from 6/7 to 7/7. Concept-synonym Hit@1
regresses on this offline fixture; vague-intent Hit@1 stays at 14.29%; no broad claim about conceptual or LLM
synonym quality is supported by this run.

## Validation

Existing Go tests and `go vet ./...` passed. Temporary CLI fixtures checked Go
receiver methods and exclusion of local variables, Python methods and named
JS/TS default exports in both auto and regex modes, bounded repetition of
symbols/synonyms, exact versus partial symbol matches, camelCase matching, and
stem coverage. Bench and eval both reject v1 bare arrays. Existing compatibility
tests were updated or removed; no new test files were added.
