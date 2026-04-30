package main

const starterConfigTemplate = `[common]
output = "context.json"
synonym_cache = ".contexting_synonyms_cache.json"
llm_model = "qwen3.5-4b"
batch_size = 0
synonyms = 4
ignore = [".git", ".venv", "site-packages", "__pycache__", "node_modules", "vendor", "dist"]
# Extra dot files to keep (merged with built-in defaults like .prettierrc, .editorconfig, etc.)
# dot_whitelist = [".env.local", ".env.production"]
verbose = true

# [llm]
# provider = "openrouter"            # openrouter (default), openai, anthropic
# endpoint = "https://openrouter.ai/api/v1/chat/completions"
# model = "qwen3.5-4b"
# api_key = "sk-or-v1-..."            # or use api_key_env for security
# api_key_env = "LLM_API_KEY"         # read key from env var instead of config file
# temperature = 0.3
# max_tokens = 512
# parallel_requests = 1

[init]
root = "."

[watch]
root = "."
debounce = "750ms"
llm = true
persist = "shutdown"
persist_interval = "45s"
search_log = true
search_log_query_max = 120 # Matches defaultSearchLogQueryMax in memory_search_server.go
max_batch_size = 0

[search]
index = "context.json"
limit = 5
min_score = 1
type = "all"
dir_summary = false
dir_limit = 5
drill_limit = 3
memory = true
runtime_file = ".contexting_runtime.json"
explain = false
json = false
show_tokens = false

[eval]
index = "context.json"
cases = "eval_cases.json"
limit = 5
min_score = 1
type = "all"
explain = false
json = false
`
