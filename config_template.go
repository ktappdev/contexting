package main

const starterConfigTemplate = `[common]
output = "context.json"
synonym_cache = ".contexting_synonyms_cache.json"
llm_model = "meta-llama/llama-3.1-8b-instruct"  # any OpenAI-compatible model name
batch_size = 15             # names per batch; 15 works well for 8B models
synonyms_min = 4            # min synonyms per name
synonyms_max = 8            # max synonyms per name
ignore = [".git", ".venv", "site-packages", "__pycache__", "node_modules", "vendor", "dist", "migrations", "pb_migrations", "alembic", "flyway"]
# Extra dot files to keep (merged with built-in defaults like .prettierrc, .editorconfig, etc.)
# dot_whitelist = [".env.local", ".env.production"]
verbose = true

[llm]
parallel_requests = 4  # concurrent LLM requests (1 = sequential)
temperature = 0.3
provider = "openrouter"
endpoint = "https://openrouter.ai/api/v1/chat/completions"
model = "meta-llama/llama-3.1-8b-instruct"
# api_key = "sk-or-v1-..."            # or use api_key_env for security
api_key_env = "OPENROUTER_API_KEY"     # read key from env var
# max_tokens = 512
# provider = "local"
# endpoint = "https://llama.kentaylor.dev/v1/chat/completions"
# model = "qwen3.5-0.8b"

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
