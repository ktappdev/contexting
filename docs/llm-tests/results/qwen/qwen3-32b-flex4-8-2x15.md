# qwen/qwen3-32b Results

- Endpoint: https://api.groq.com/openai/v1/chat/completions
- Config: 2x15 parallel | 4-8 (flex) synonyms
- Runs: 5
- Source: /Users/kentaylor/developer/vault/vaultgy/context.json

## Summary

- Valid JSON: **0/5**

## Runs

| Run | Valid | Names | Synonyms | Tokens | Time |
|-----|-------|-------|----------|--------|------|
| 1 | NO | — | — | 2777 | 3.1s |
| 2 | NO | — | — | 0 | 0.4s |
| 3 | NO | — | — | 1994 | 3.8s |
| 4 | NO | — | — | 0 | 0.4s |
| 5 | NO | — | — | 0 | 0.4s |

## Failures

- Run 1: `Expecting value: line 1 column 1 (char 0); Expecting value: line 1 column 1 (char 0)`
- Run 2: `Rate limit reached for model `qwen/qwen3-32b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` servi; Rate limit reached for model `qwen/qwen3-32b` in`
- Run 3: `Rate limit reached for model `qwen/qwen3-32b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` servi; Expecting value: line 1 column 1 (char 0)`
- Run 4: `Rate limit reached for model `qwen/qwen3-32b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` servi; Rate limit reached for model `qwen/qwen3-32b` in`
- Run 5: `Rate limit reached for model `qwen/qwen3-32b` in organization `org_01hvyhq6xtfj1t0xvrd1y751hw` servi; Rate limit reached for model `qwen/qwen3-32b` in`
