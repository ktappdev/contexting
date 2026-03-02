package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLoadContextingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "context.toml")
	content := `
[common]
output = "out/context.json"
llm_model = "openrouter/free"
batch_size = 12
ignore = ["dist", "build"]
verbose = true

[watch]
debounce = "2s"
search_log = false
search_log_query_max = 77

[search]
index = "ctx.json"
limit = 7
dir_summary = true
dir_limit = 6
drill_limit = 2
show_tokens = true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadContextingConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Common.OutputPath != "out/context.json" {
		t.Fatalf("unexpected output: %s", cfg.Common.OutputPath)
	}
	if cfg.Common.BatchSize != 12 {
		t.Fatalf("unexpected batch size: %d", cfg.Common.BatchSize)
	}
	if cfg.Watch.Debounce != "2s" {
		t.Fatalf("unexpected debounce: %s", cfg.Watch.Debounce)
	}
	if cfg.Watch.SearchLog == nil || *cfg.Watch.SearchLog {
		t.Fatalf("expected watch.search_log=false")
	}
	if cfg.Watch.SearchLogQueryMax != 77 {
		t.Fatalf("expected watch.search_log_query_max=77, got %d", cfg.Watch.SearchLogQueryMax)
	}
	if cfg.Search.Limit != 7 {
		t.Fatalf("unexpected search limit: %d", cfg.Search.Limit)
	}
	if cfg.Search.DirSummary == nil || !*cfg.Search.DirSummary {
		t.Fatalf("expected dir_summary=true")
	}
	if cfg.Search.DirLimit != 6 {
		t.Fatalf("expected dir_limit=6, got %d", cfg.Search.DirLimit)
	}
	if cfg.Search.DrillLimit != 2 {
		t.Fatalf("expected drill_limit=2, got %d", cfg.Search.DrillLimit)
	}
	if cfg.Search.ShowTokens == nil || !*cfg.Search.ShowTokens {
		t.Fatalf("expected show_tokens=true")
	}
}

func TestApplyCommonConfigRespectsCLIOverride(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	flags := CommonFlags{}
	cmd.Flags().StringVarP(&flags.OutputPath, "output", "o", "context.json", "")
	cmd.Flags().IntVar(&flags.BatchSize, "batch-size", 8, "")
	cmd.Flags().StringSliceVar(&flags.ExtraIgnores, "ignore", nil, "")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "")
	cmd.Flags().StringVar(&flags.SynonymCache, "synonym-cache", "cache.json", "")
	cmd.Flags().StringVar(&flags.Model, "llm-model", defaultModel, "")
	cmd.Flags().StringVar(&flags.APIKey, "api-key", "", "")
	cmd.Flags().IntVar(&flags.SynonymsPerName, "synonyms", 4, "")

	if err := cmd.Flags().Set("output", "cli.json"); err != nil {
		t.Fatalf("set output: %v", err)
	}

	verbose := true
	applyCommonConfig(cmd, &flags, CommonConfig{
		OutputPath:      "config.json",
		BatchSize:       20,
		ExtraIgnores:    []string{"dist"},
		Verbose:         &verbose,
		SynonymsPerName: 6,
	})

	if flags.OutputPath != "cli.json" {
		t.Fatalf("cli flag should win, got %s", flags.OutputPath)
	}
	if flags.BatchSize != 20 {
		t.Fatalf("expected config batch size applied, got %d", flags.BatchSize)
	}
	if len(flags.ExtraIgnores) != 1 || flags.ExtraIgnores[0] != "dist" {
		t.Fatalf("expected config ignore applied, got %v", flags.ExtraIgnores)
	}
	if !flags.Verbose {
		t.Fatalf("expected verbose from config")
	}
}

func TestWatchDebounceDuration(t *testing.T) {
	cfg := WatchConfig{Debounce: "1500ms"}
	d, err := cfg.DebounceDuration()
	if err != nil {
		t.Fatalf("duration parse failed: %v", err)
	}
	if d.Milliseconds() != 1500 {
		t.Fatalf("unexpected duration: %v", d)
	}
}

func TestWatchPersistIntervalDuration(t *testing.T) {
	cfg := WatchConfig{PersistInterval: "45s"}
	d, err := cfg.PersistIntervalDuration()
	if err != nil {
		t.Fatalf("persist interval parse failed: %v", err)
	}
	if d.Seconds() != 45 {
		t.Fatalf("unexpected persist interval: %v", d)
	}
}
