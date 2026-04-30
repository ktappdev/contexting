# LLM Model Testing for Contexting Synonyms

Testing synonym generation reliability for different models.

## Endpoint

```
https://llama.kentaylor.dev/v1/chat/completions
```

No API key required. OpenAI-compatible.

## Prompt Format

Matches contexting's actual prompt in `openrouter.go`:

**System:**
```
You are a helpful assistant. For each folder or file name in the list, generate exactly N plausible alternative words or short phrases a developer might use when searching for that file in a codebase. Return ONLY a valid JSON object where each key is an exact filename from the input list and each value is an array of N synonym strings. Example: {"auth.go": ["login", "authentication", "session"], "config": ["settings", "configuration", "options"]}. No markdown, no prose, no extra text.
```

**User:**
```
File and folder names:
name1
name2
...
```

## Metrics

| Metric | Description |
|--------|-------------|
| Valid JSON | Does it return parseable JSON? (raw_decode) |
| Names returned | How many of the input names appear in output? |
| Missing names | Which input names were dropped? |
| Extra names | Did it hallucinate names not in input? |
| Synonyms per name | min/max/avg count (target: N) |
| Total tokens | Cost/speed indicator |
| Finish reason | `stop` = model chose to end, `length` = hit max_tokens |

## How to Run

```bash
cd /Users/kentaylor/developer/llm-search-thing/contexting/docs/llm-tests
python3 test_synonyms.py --model MODEL --batch-size 60 --runs 5
```

## File Structure

```
llm-tests/
  README.md           # this file
  test_synonyms.py    # test script
  results/
    qwen3.5-0.8b.md   # results per model
    qwen3.5-4b.md
    ...
```
