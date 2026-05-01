# openai/gpt-oss-20b Results

- Endpoint: https://api.groq.com/openai/v1/chat/completions
- Config: 4x15 parallel | 4-8 (flex) synonyms
- Runs: 5
- Source: /Users/kentaylor/developer/vault/vaultgy/ctx_index.json

## Summary

- Valid JSON: **2/5**
- Avg names returned: 60.0/60
- Avg synonyms: 7.0
- Avg tokens: 7036
- Avg wall time: 2.6s

## Runs

| Run | Valid | Names | Synonyms | Tokens | Time |
|-----|-------|-------|----------|--------|------|
| 1 | YES | 60/60 | 5-8 avg=6.2 | 6708 | 2.8s |
| 2 | YES | 60/60 | 5-8 avg=7.7 | 7365 | 2.4s |
| 3 | NO | — | — | 0 | 0.5s |
| 4 | NO | — | — | 0 | 0.3s |
| 5 | NO | — | — | 0 | 0.3s |

## Failures

- Run 3: `Rate limit reached for model `openai/gpt-oss-20b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` s; Rate limit reached for model `openai/gpt-oss-20b`
- Run 4: `Rate limit reached for model `openai/gpt-oss-20b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` s; Rate limit reached for model `openai/gpt-oss-20b`
- Run 5: `Rate limit reached for model `openai/gpt-oss-20b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` s; Rate limit reached for model `openai/gpt-oss-20b`
