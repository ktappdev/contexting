# meta-llama/llama-3.1-8b-instruct Results

- Endpoint: https://openrouter.ai/api/v1/chat/completions
- Config: 4x15 parallel | 4-8 (flex) synonyms
- Runs: 5
- Source: /Users/kentaylor/developer/vault/vaultgy/context.json

## Summary

- Valid JSON: **5/5**
- Avg names returned: 59.6/60
- Avg synonyms: 3.9
- Avg tokens: 2443
- Avg wall time: 9.7s

## Runs

| Run | Valid | Names | Synonyms | Tokens | Time |
|-----|-------|-------|----------|--------|------|
| 1 | YES | 59/60 | 3-6 avg=4.1 | 2446 | 20.3s |
| 2 | YES | 59/60 | 2-5 avg=3.6 | 2302 | 6.8s |
| 3 | YES | 60/60 | 1-6 avg=3.8 | 2540 | 5.4s |
| 4 | YES | 60/60 | 3-6 avg=4.0 | 2405 | 8.1s |
| 5 | YES | 60/60 | 4-5 avg=4.2 | 2520 | 8.1s |
