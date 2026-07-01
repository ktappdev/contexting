# Benchmark Feedback — contexting search quality

Ran `contexting bench` with a 45-case file across a multi-language monorepo (Go + TypeScript + Svelte). 4 engines compared: contexting, find, grep, combined. Overall results:

| Engine | Hit@1 | Hit@3 | Hit@5 | Avg Time | Avg Tokens | Noise |
|---|---|---|---|---|---|---|
| contexting | 0.36 | 0.58 | 0.62 | 1ms | 122 | 0.86 |
| find | 0.20 | 0.29 | 0.33 | 5ms | 562 | 0.74 |
| grep | 0.02 | 0.09 | 0.16 | 20ms | 1922 | 0.95 |
| combined | 0.02 | 0.09 | 0.16 | 24ms | 2068 | 0.95 |

contexting wins overall on accuracy and token efficiency (16× less output than grep). Below are areas where it underperforms its potential, with suggested improvements.

---

## 1. Boost exact-filename matches to near-max score

**Observation:** When the query *is* a filename (e.g. `types.go`, `ffmpeg.go`), find scores Hit@1 = 1.00 while contexting scores 0.50. contexting sometimes ranks the exact-match file at position 4–8 because semantic scoring dilutes the literal signal.

**Suggestion:** If a query token exactly matches a file's basename (stem or full name), apply a strong score boost — enough to guarantee top-3 placement. Exact filename match is the highest-confidence intent signal; it should not be outranked by synonym/path fuzzy matches.

**Risk:** Minimal. This only fires on literal matches, which are almost always the user's intent in this case.

---

## 2. Improve vague-intent / conceptual queries

**Observation:** The "vague-intent" category (queries with no obvious keyword match, e.g. "how does the tool know which words to mute", "how does the backend remember users between requests") scored Hit@1 = 0.00 across *all* engines. contexting's synonym expansion didn't bridge the gap between user vocabulary and code vocabulary in these cases.

**Suggestion:** Expand synonym generation to cover verb/action synonyms, not just noun/concept synonyms. "mute" → `detect`/`profanity`/`filter`, "remember" → `session`/`auth`/`cache`/`middleware`. Currently synonyms seem biased toward nouns (file purposes) over verbs (what the code *does*). Consider prompting the LLM to generate action-oriented synonyms: "what verbs would a developer use to describe this file's responsibility?"

---

## 3. Path-intent: symbol extraction should feed path scoring

**Observation:** Path-intent queries (e.g. "where the ffmpeg filter string is built for muting words") scored Hit@1 = 0.25. The target file contains a highly relevant exported symbol (`BuildFilterString`), but contexting didn't surface it — likely because the query words don't match the symbol name directly.

**Suggestion:** When scoring path-intent queries, include partial/fuzzy symbol-name matches in the path score. A query like "filter string is built" should boost files containing symbols named `Build*Filter*` or `*Filter*String*`. Symbol names are high-signal metadata — they should contribute to path ranking, not just symbol-lookup queries.

---

## 4. Reduce noise ratio

**Observation:** contexting's overall noise ratio is 0.86 — meaning ~6 of 10 returned results are non-matches even on successful queries. For an LLM-context tool, this is significant token waste.

**Suggestion:** Consider a sharper score cutoff or a "confidence gap" heuristic: if there's a large score drop between result N and N+1, truncate the list rather than padding to `--limit`. Returning 3 high-confidence results is more useful for LLM context than 10 results where 7 are noise.

---

## 5. combined engine: filter or drop

**Observation:** The `combined` engine (union of find + grep) scored *below* contexting alone on every category, while returning 17× more tokens. grep's broad content match floods the result set with irrelevant files, and the union doesn't rerank by relevance.

**Suggestion:** Either (a) drop `combined` as a default engine, or (b) rerank the union through contexting's scoring instead of returning alphabetical order. A raw union without relevance ranking provides no value over running the engines separately.

---

## What's working well

- **Symbol lookup** (Hit@1 = 0.71): exported function/type names reliably rank #1. This is contexting's strongest category and a clear differentiator from find/grep.
- **Concept-synonym** (Hit@1 = 0.50): noun-oriented synonyms bridge vocabulary gaps effectively ("helper binary" → installer, "rate limits" → rate limiter).
- **Narrow-scope disambiguation** (Hit@5 = 1.00): "X not Y" queries consistently land the right file in top 5.
- **Token efficiency**: 122 avg tokens vs 1922 (grep) — critical advantage for LLM context budgets.
- **Speed**: 1ms avg query — negligible overhead.

---

*Case file and raw benchmark output available on request.*
