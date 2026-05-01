# qwen3.5-0.8b Results

- Endpoint: https://llama.kentaylor.dev/v1/chat/completions
- Config: 4x15 parallel | 4-8 (flex) synonyms
- Runs: 5
- Source: /Users/kentaylor/developer/vault/vaultgy/ctx_index.json

## Summary

- Valid JSON: **2/5**
- Avg names returned: 60.0/60
- Avg synonyms: 7.2
- Avg tokens: 2787
- Avg wall time: 15.9s

## Runs

| Run | Valid | Names | Synonyms | Tokens | Time |
|-----|-------|-------|----------|--------|------|
| 1 | YES | 60/60 | 3-32 avg=8.9 | 2957 | 17.6s |
| 2 | NO | — | — | 1555 | 125.4s |
| 3 | NO | — | — | 1912 | 8.8s |
| 4 | NO | — | — | 2186 | 9.5s |
| 5 | YES | 60/60 | 3-11 avg=5.4 | 2617 | 14.1s |

## Failures

- Run 2: `API returned non-JSON: error code: 524`
- Run 3: `Expecting ',' delimiter: line 1 column 974 (char 973)`
- Run 4: `Expecting ',' delimiter: line 1 column 864 (char 863)`
