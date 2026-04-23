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
	Verbose         bool
	ExtraIgnores    []string
}

func (c *CommonFlags) normalize() {
	if c.BatchSize <= 0 {
		c.BatchSize = 8
	}
	if c.SynonymsPerName <= 0 {
		c.SynonymsPerName = defaultSynonyms
	}
	if c.SynonymCache == "" {
		c.SynonymCache = ".contexting_synonyms_cache.json"
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
