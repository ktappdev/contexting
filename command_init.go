package main

import (
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	flags := CommonFlags{}

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Build context index and write context JSON",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadContextingConfig(configPath)
			if err != nil {
				return err
			}
			applyCommonConfig(cmd, &flags, cfg.Common)
			flags.normalize()

			rootPath := "."
			if len(args) == 1 {
				rootPath = args[0]
			} else if cfg.Init.RootPath != "" {
				rootPath = cfg.Init.RootPath
			}
			absRoot, err := filepath.Abs(rootPath)
			if err != nil {
				return err
			}
			outputPath := resolveProjectPath(absRoot, flags.OutputPath)
			cachePath := resolveProjectPath(absRoot, flags.SynonymCache)

			ignored, err := BuildIgnoreMapForRoot(absRoot, flags.ExtraIgnores)
			if err != nil {
				return err
			}
			apiKey := resolveAPIKey(flags.APIKey)
			cache, err := LoadSynonymCache(cachePath)
			if err != nil {
				return err
			}
			if apiKey == "" {
				logWarnf("OPENROUTER_API_KEY not set and --api-key not provided; continuing without synonyms")
			}
			ctx, stop := signalAwareContext()
			defer stop()

			result, err := BuildIndex(BuildOptions{
				Ctx:             ctx,
				RootPath:        rootPath,
				IgnoredPaths:    ignored,
				APIKey:          apiKey,
				Model:           flags.Model,
				BatchSize:       flags.BatchSize,
				SynonymsPerName: flags.SynonymsPerName,
				SynonymCache:    cache,
				MaxBatchSize:    cfg.Watch.MaxBatchSize,
			})
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}

			emitSynonymWarning(result.SynonymError)
			if err := SaveSynonymCache(cachePath, result.SynonymCache); err != nil {
				return err
			}
			if err := SaveContextIndex(outputPath, result.Index); err != nil {
				return err
			}

			logInfof("Indexed %d nodes (%d files, %d directories). Synonyms on %d nodes.", result.Stats.TotalNodes, result.Stats.TotalFiles, result.Stats.TotalDirs, result.Stats.SynonymNodes)
			logInfof("Collected %d unique names. Wrote %s", result.Stats.CollectedNames, outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.OutputPath, "output", "o", "context.json", "Output JSON path")
	cmd.Flags().StringVar(&flags.Model, "llm-model", defaultModel, "LLM model used for synonym generation")
	cmd.Flags().StringVar(&flags.APIKey, "api-key", "", "OpenRouter API key (falls back to OPENROUTER_API_KEY)")
	cmd.Flags().IntVar(&flags.BatchSize, "batch-size", 8, "Names per LLM request")
	cmd.Flags().IntVar(&flags.SynonymsPerName, "synonyms", 4, "Desired synonyms per name")
	cmd.Flags().StringVar(&flags.SynonymCache, "synonym-cache", ".contexting_synonyms_cache.json", "Path to persistent synonym cache JSON")
	cmd.Flags().StringSliceVar(&flags.ExtraIgnores, "ignore", nil, "Additional ignore entries (name or relative path)")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")

	return cmd
}
