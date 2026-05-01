# qwen3.5-0.8b Results

## 60 names, sequential

- Batch size: 60
- Synonyms per name: 4
- Runs: 5
- Source: /Users/kentaylor/developer/vault/vaultgy/ctx_index.json

### Summary

- Valid JSON: **1/5**
- Avg names returned (valid runs): 60.0/60
- Avg tokens (valid): 1673

### Runs

| Run | Valid | Names | Synonyms | Missing | Extra | Tokens |
|-----|-------|-------|----------|---------|-------|--------|
| 1 | NO | — | — | — | — | 1494 |
| 2 | NO | — | — | — | — | 1676 |
| 3 | NO | — | — | — | — | 1935 |
| 4 | YES | 60/60 | 3-4 avg=4.0 | 1 | 1 | 1673 |
| 5 | NO | — | — | — | — | 1668 |

---

## 30 names, 2 parallel batches (60 total)

- Batch size: 30 × 2 parallel
- Synonyms per name: 4
- Runs: 5

### Summary

- Valid JSON: **1/5** (1 partial, 1 full, 3 failed)

### Runs

| Run | Valid | Names | Synonyms | Missing | Extra | Tokens |
|-----|-------|-------|----------|---------|-------|--------|
| 1 | PARTIAL | 30/60 | — | — | — | 1703 |
| 2 | YES | 60/60 | 3-4 avg=3.5 | 1 | 1 | 1703 |
| 3 | NO | — | — | — | — | — |
| 4 | NO | — | — | — | — | — |
| 5 | NO | — | — | — | — | — |

### Notes

- Common failure: `Expecting ',' delimiter` — drops commas between JSON entries
- Model finishes with `stop` even when JSON is incomplete/invalid
- Not reliable at any batch size tested
- **Verdict: not suitable for contexting synonym generation**
