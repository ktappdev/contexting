# openai/gpt-oss-20b Results

- Endpoint: https://api.groq.com/openai/v1/chat/completions
- Config: 2x15 parallel | 4-8 (flex) synonyms
- Runs: 5
- Source: /Users/kentaylor/developer/vault/vaultgy/ctx_index.json

## Summary

- Valid JSON: **1/5**
- Avg names returned: 30.0/30
- Avg synonyms: 6.8
- Avg tokens: 3185
- Avg wall time: 2.1s

## Runs

| Run | Valid | Names | Synonyms | Tokens | Time |
|-----|-------|-------|----------|--------|------|
| 1 | NO | — | — | 1479 | 1.7s |
| 2 | YES | 30/30 | 5-8 avg=6.8 | 3185 | 2.1s |
| 3 | NO | — | — | 0 | 0.3s |
| 4 | NO | — | — | 0 | 0.3s |
| 5 | NO | — | — | 0 | 0.4s |

## Failures

- Run 1: `Rate limit reached for model `openai/gpt-oss-20b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` s`
- Run 3: `Rate limit reached for model `openai/gpt-oss-20b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` s; Rate limit reached for model `openai/gpt-oss-20b`
- Run 4: `Rate limit reached for model `openai/gpt-oss-20b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` s; Rate limit reached for model `openai/gpt-oss-20b`
- Run 5: `Rate limit reached for model `openai/gpt-oss-20b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` s; Rate limit reached for model `openai/gpt-oss-20b`
