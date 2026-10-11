package contexting

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

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
	SymbolExtractor string
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
		c.SynonymsMax = defaultSynonymsMax
	}
	if c.SynonymsMin > c.SynonymsMax {
		c.SynonymsMin = c.SynonymsMax
	}
	if c.SynonymCache == "" {
		c.SynonymCache = ".ctxt/ctx_cache.json"
	}
}

// resolveStartupConfigPath anchors the default config path to the resolved project
// root so ctxt watch/mcp work regardless of the process working directory. When a
// root argument is given, the default ".ctxt/ctx_config.toml" resolves to that root;
// otherwise it resolves to the working directory as before. An explicitly supplied
// --config keeps its previous behavior: it resolves against the working directory.
func resolveStartupConfigPath(configPath string, rootArg string, configOverridden bool) (string, error) {
	if configOverridden {
		if configPath == "" {
			return "", nil
		}
		return filepath.Abs(configPath)
	}
	configRoot := "."
	if rootArg != "" {
		configRoot = rootArg
	}
	absRoot, err := filepath.Abs(configRoot)
	if err != nil {
		return "", fmt.Errorf("resolve config root: %w", err)
	}
	return resolveProjectPath(absRoot, configPath), nil
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
	if temperature == 0 {
		temperature = 0.9
	}
	maxTokens = llmCfg.MaxTokens
	if offlineMode {
		apiKey = ""
	}
	return
}

func maskAPIKey(key string) string {
	if key == "" {
		return "[not set]"
	}
	return "[set]"
}

func endpointForLog(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "[invalid]"
	}
	return parsed.Scheme + "://" + parsed.Host
}

func emitSynonymWarning(err error) {
	if err == nil {
		return
	}
	if isCanceledError(err) {
		return
	}
	LogWarnf("synonym generation failed, continuing without synonyms: %v", err)
}
