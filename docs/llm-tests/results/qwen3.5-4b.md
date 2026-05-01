# qwen3.5-4b Results

## 60 names, sequential

- Batch size: 60
- Synonyms per name: 4
- Runs: 5
- Source: /Users/kentaylor/developer/vault/vaultgy/ctx_index.json

### Summary

- Valid JSON: **3/5**
- Avg names returned (valid runs): 59.7/60
- Avg tokens (valid): 2198

### Runs

| Run | Valid | Names | Synonyms | Missing | Extra | Tokens |
|-----|-------|-------|----------|---------|-------|--------|
| 1 | YES | 60/60 | 4-4 avg=4.0 | 0 | 0 | 2076 |
| 2 | NO | — | — | — | — | 2467 |
| 3 | YES | 60/60 | 4-4 avg=4.0 | 2 | 2 | 2496 |
| 4 | YES | 59/60 | 4-4 avg=4.0 | 1 | 0 | 2022 |
| 5 | NO | — | — | — | — | 2150 |

---

## 30 names, sequential

- Batch size: 30
- Synonyms per name: 4
- Runs: 5

### Summary

- Valid JSON: **5/5**
- Avg names returned: 29.8/30
- Avg tokens: 1418

### Runs

| Run | Valid | Names | Synonyms | Missing | Extra | Tokens |
|-----|-------|-------|----------|---------|-------|--------|
| 1 | YES | 30/30 | 4-4 avg=4.0 | 1 | 1 | 1479 |
| 2 | YES | 30/30 | 4-4 avg=4.0 | 1 | 1 | 1537 |
| 3 | YES | 30/30 | 4-4 avg=4.0 | 0 | 0 | 1367 |
| 4 | YES | 29/30 | 4-4 avg=4.0 | 1 | 0 | 1168 |
| 5 | YES | 30/30 | 4-4 avg=4.0 | 1 | 1 | 1537 |

---

## 30 names, 2 parallel batches (60 total)

- Batch size: 30 × 2 parallel
- Synonyms per name: 4
- Runs: 5

### Summary

- Valid JSON: **5/5**
- Avg names returned: 59.8/60
- Avg tokens: 2233

### Runs

| Run | Valid | Names | Synonyms | Missing | Extra | Tokens |
|-----|-------|-------|----------|---------|-------|--------|
| 1 | YES | 60/60 | 4-4 avg=4.0 | 1 | 1 | 2465 |
| 2 | YES | 59/60 | 4-4 avg=4.0 | 1 | 0 | 2058 |
| 3 | YES | 60/60 | 4-4 avg=4.0 | 1 | 1 | 2463 |
| 4 | YES | 60/60 | 4-4 avg=4.0 | 0 | 0 | 2105 |
| 5 | YES | 60/60 | 4-4 avg=4.0 | 2 | 2 | 2074 |

### Notes

- Run 4 was perfect: 60/60 names, 0 missing, 0 extra
- Minor quirks: `.env.example` → `env.example` (dot strip), `.tsx` → `.svelte` swap
- Both are model-level, not format issues
- **Best config: 4b @ 30 names parallel = reliable**
