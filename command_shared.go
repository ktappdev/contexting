package main

import "os"

type CommonFlags struct {
	OutputPath      string
	SynonymCache    string
	Model           string
	APIKey          string
	Endpoint        string
	BatchSize       int
	SynonymsPerName int
	SynonymsMin     int
	SynonymsMax     int
	Verbose         bool
	ExtraIgnores    []string
}

func (c *CommonFlags) normalize() {
	// BatchSize 0 means smart batching (up to 1000 names per request)
	if c.SynonymsPerName <= 0 {
		c.SynonymsPerName = defaultSynonyms
	}
	// Resolve synonyms min/max: if set, use them; otherwise fall back to SynonymsPerName
	if c.SynonymsMin <= 0 {
		c.SynonymsMin = c.SynonymsPerName
	}
	if c.SynonymsMax <= 0 {
		c.SynonymsMax = c.SynonymsPerName
	}
	if c.SynonymsMin > c.SynonymsMax {
		c.SynonymsMin = c.SynonymsMax
	}
	if c.SynonymCache == "" {
		c.SynonymCache = ".ctx/ctx_cache.json"
	}
}

func resolveAPIKey(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	key, err := GetAPIKey()
	if err != nil {
		return ""
	}
	return key
}

func resolveLLMConfig(flags CommonFlags, llmCfg LLMConfig) (endpoint, model, apiKey string, temperature float64, maxTokens int, provider string) {
	provider = llmCfg.Provider
	if provider == "" {
		provider = "openrouter"
	}
	endpoint = flags.Endpoint
	if endpoint == "" {
		endpoint = llmCfg.Endpoint
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	model = flags.Model
	if model == "" {
		model = llmCfg.Model
	}
	if model == "" {
		model = defaultModel
	}
	apiKey = flags.APIKey
	if apiKey == "" {
		apiKey = llmCfg.APIKey
	}
	if apiKey == "" {
		keyEnv := llmCfg.APIKeyEnv
		if keyEnv != "" {
			apiKey = os.Getenv(keyEnv)
		}
	}
	if apiKey == "" {
		apiKey = os.Getenv("LLM_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	temperature = llmCfg.Temperature
	maxTokens = llmCfg.MaxTokens
	return
}

func maskAPIKey(key string) string {
	if key == "" {
		return "[not set]"
	}
	if len(key) <= 8 {
		return key[:max(1, len(key)/2)] + "..."
	}
	return key[:8] + "..."
}

func emitSynonymWarning(err error) {
	if err == nil {
		return
	}
	if isCanceledError(err) {
		return
	}
	logWarnf("synonym generation failed, continuing without synonyms: %v", err)
}
