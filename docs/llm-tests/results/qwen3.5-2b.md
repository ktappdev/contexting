# qwen3.5-2b Results

## 60 names, sequential

- Batch size: 60
- Synonyms per name: 4
- Runs: 5
- Source: /Users/kentaylor/developer/vault/vaultgy/ctx_index.json

### Summary

- Valid JSON: **5/5**
- Avg names returned: 36.4/60
- Avg tokens: 1945

### Runs

| Run | Valid | Names | Synonyms | Missing | Extra | Tokens |
|-----|-------|-------|----------|---------|-------|--------|
| 1 | YES | 1/60 | 4-4 avg=4.0 | 60 | 1 | 1957 |
| 2 | YES | 60/60 | 4-4 avg=4.0 | 1 | 1 | 1911 |
| 3 | YES | 60/60 | 4-4 avg=4.0 | 1 | 1 | 1921 |
| 4 | YES | 1/60 | 4-4 avg=4.0 | 60 | 1 | 2005 |
| 5 | YES | 60/60 | 4-4 avg=4.0 | 1 | 1 | 1929 |

---

## 30 names, 2 parallel batches (60 total)

- Batch size: 30 × 2 parallel
- Synonyms per name: 4
- Runs: 5

### Summary

- Valid JSON: **5/5** (but collapse bug on most)

### Runs

| Run | Valid | Names | Missing | Extra | Tokens |
|-----|-------|-------|---------|-------|--------|
| 1 | PARTIAL | 31/60 | 30 | 1 | 1866 |
| 2 | PARTIAL | 2/60 | 59 | 1 | 1967 |
| 3 | PARTIAL | 2/60 | 59 | 1 | 1996 |
| 4 | PARTIAL | 2/60 | 59 | 1 | 2002 |
| 5 | PARTIAL | 2/60 | 59 | 1 | 2013 |

### Notes

- Always produces valid JSON (5/5)
- Bug: collapses most/all names into 1-2 keys
- Good runs return perfect 4 synonyms, but rare
- **Verdict: valid JSON but unreliable name coverage**
