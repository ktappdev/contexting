package main

type CommonFlags struct {
	OutputPath      string
	SynonymCache    string
	Model           string
	APIKey          string
	BatchSize       int
	SynonymsPerName int
	Verbose         bool
	ExtraIgnores    []string
}

func (c *CommonFlags) normalize() {
	if c.Model == "" {
		c.Model = defaultModel
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 8
	}
	if c.SynonymsPerName <= 0 {
		c.SynonymsPerName = 4
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

func emitSynonymWarning(err error) {
	if err == nil {
		return
	}
	if isCanceledError(err) {
		return
	}
	logWarnf("synonym generation failed, continuing without synonyms: %v", err)
}
