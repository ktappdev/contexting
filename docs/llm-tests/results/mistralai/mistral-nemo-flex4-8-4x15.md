# mistralai/mistral-nemo Results

- Endpoint: https://openrouter.ai/api/v1/chat/completions
- Config: 4x15 parallel | 4-8 (flex) synonyms
- Runs: 5
- Source: /Users/kentaylor/developer/vault/vaultgy/context.json

## Summary

- Valid JSON: **2/5**
- Avg names returned: 59.5/60
- Avg synonyms: 3.2
- Avg tokens: 2433
- Avg wall time: 25.8s

## Runs

| Run | Valid | Names | Synonyms | Tokens | Time |
|-----|-------|-------|----------|--------|------|
| 1 | NO | — | — | 2649 | 65.9s |
| 2 | NO | — | — | 2647 | 5.5s |
| 3 | YES | 60/60 | 3-4 avg=3.2 | 2534 | 26.3s |
| 4 | NO | — | — | 2202 | 28.7s |
| 5 | YES | 59/60 | 3-4 avg=3.2 | 2332 | 25.3s |

## Failures

- Run 1: `Expecting value: line 1 column 1 (char 0); Expecting value: line 1 column 1 (char 0)`
- Run 2: `Expecting value: line 1 column 1 (char 0)`
- Run 4: `Expecting value: line 1 column 1 (char 0)`
