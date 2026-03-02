package main

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type ContextingConfig struct {
	Common CommonConfig `toml:"common"`
	Init   InitConfig   `toml:"init"`
	Watch  WatchConfig  `toml:"watch"`
	Search SearchConfig `toml:"search"`
	Eval   EvalConfig   `toml:"eval"`
}

type CommonConfig struct {
	OutputPath      string   `toml:"output"`
	SynonymCache    string   `toml:"synonym_cache"`
	Model           string   `toml:"llm_model"`
	APIKey          string   `toml:"api_key"`
	BatchSize       int      `toml:"batch_size"`
	SynonymsPerName int      `toml:"synonyms"`
	Verbose         *bool    `toml:"verbose"`
	ExtraIgnores    []string `toml:"ignore"`
}

type InitConfig struct {
	RootPath string `toml:"root"`
}

type WatchConfig struct {
	RootPath          string `toml:"root"`
	Debounce          string `toml:"debounce"`
	UseLLM            *bool  `toml:"llm"`
	Persist           string `toml:"persist"`
	PersistInterval   string `toml:"persist_interval"`
	SearchLog         *bool  `toml:"search_log"`
	SearchLogQueryMax int    `toml:"search_log_query_max"`
}

type SearchConfig struct {
	IndexPath   string `toml:"index"`
	Limit       int    `toml:"limit"`
	MinScore    int    `toml:"min_score"`
	TypeFilter  string `toml:"type"`
	DirSummary  *bool  `toml:"dir_summary"`
	DirLimit    int    `toml:"dir_limit"`
	DrillLimit  int    `toml:"drill_limit"`
	UseMemory   *bool  `toml:"memory"`
	RuntimeFile string `toml:"runtime_file"`
	Explain     *bool  `toml:"explain"`
	JSON        *bool  `toml:"json"`
	ShowTokens  *bool  `toml:"show_tokens"`
}

type EvalConfig struct {
	IndexPath  string `toml:"index"`
	CasesPath  string `toml:"cases"`
	Limit      int    `toml:"limit"`
	MinScore   int    `toml:"min_score"`
	TypeFilter string `toml:"type"`
	Explain    *bool  `toml:"explain"`
	JSON       *bool  `toml:"json"`
}

func LoadContextingConfig(path string) (*ContextingConfig, error) {
	cfg := &ContextingConfig{}
	if path == "" {
		return cfg, nil
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("stat config %s: %w", path, err)
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}
	return cfg, nil
}

func (w WatchConfig) DebounceDuration() (time.Duration, error) {
	if w.Debounce == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(w.Debounce)
	if err != nil {
		return 0, fmt.Errorf("invalid watch.debounce %q: %w", w.Debounce, err)
	}
	return d, nil
}

func (w WatchConfig) PersistIntervalDuration() (time.Duration, error) {
	if w.PersistInterval == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(w.PersistInterval)
	if err != nil {
		return 0, fmt.Errorf("invalid watch.persist_interval %q: %w", w.PersistInterval, err)
	}
	return d, nil
}
