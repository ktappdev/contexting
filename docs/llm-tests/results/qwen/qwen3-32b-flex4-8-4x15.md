# qwen/qwen3-32b Results

- Endpoint: https://api.groq.com/openai/v1/chat/completions
- Config: 4x15 parallel | 4-8 (flex) synonyms
- Runs: 5
- Source: /Users/kentaylor/developer/vault/vaultgy/ctx_index.json

## Summary

- Valid JSON: **0/5**

## Runs

| Run | Valid | Names | Synonyms | Tokens | Time |
|-----|-------|-------|----------|--------|------|
| 1 | NO | — | — | 6130 | 3.8s |
| 2 | NO | — | — | 1036 | 3.1s |
| 3 | NO | — | — | 0 | 0.3s |
| 4 | NO | — | — | 0 | 0.7s |
| 5 | NO | — | — | 0 | 0.3s |

## Failures

- Run 1: `Expecting value: line 1 column 1 (char 0); Expecting value: line 1 column 1 (char 0); Expecting value: line 1 column 1 (char 0); Expecting value: line`
- Run 2: `Rate limit reached for model `qwen/qwen3-32b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` servi; Expecting value: line 1 column 1 (char 0); Rate `
- Run 3: `Rate limit reached for model `qwen/qwen3-32b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` servi; Rate limit reached for model `qwen/qwen3-32b` in`
- Run 4: `Rate limit reached for model `qwen/qwen3-32b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` servi; Rate limit reached for model `qwen/qwen3-32b` in`
- Run 5: `Rate limit reached for model `qwen/qwen3-32b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` servi; Rate limit reached for model `qwen/qwen3-32b` in`
