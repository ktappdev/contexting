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
			llmEndpoint, llmModel, llmKey, llmTemp, llmMaxTokens, llmProvider := resolveLLMConfig(flags, cfg.LLM)
			logInfof("LLM: provider=%s model=%s endpoint=%s api_key=%s", llmProvider, llmModel, llmEndpoint, maskAPIKey(llmKey))
			cache, err := LoadSynonymCache(cachePath)
			if err != nil {
				return err
			}
			if llmKey == "" {
				logWarnf("LLM API key not configured; continuing without synonyms")
			}
			ctx, stop := signalAwareContext()
			defer stop()

			result, err := BuildIndex(BuildOptions{
				Ctx:              ctx,
				RootPath:         rootPath,
				IgnoredPaths:     ignored,
				APIKey:           llmKey,
				Model:            llmModel,
				BatchSize:        flags.BatchSize,
				SynonymsPerName:  flags.SynonymsPerName,
				SynonymCache:     cache,
				MaxBatchSize:     cfg.Watch.MaxBatchSize,
				Endpoint:         llmEndpoint,
				Temperature:      llmTemp,
				MaxTokens:        llmMaxTokens,
				ParallelRequests: cfg.LLM.ParallelRequests,
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
	cmd.Flags().StringVar(&flags.Model, "llm-model", "", "LLM model used for synonym generation")
	cmd.Flags().StringVar(&flags.APIKey, "api-key", "", "LLM API key (falls back to config api_key_env, LLM_API_KEY, OPENROUTER_API_KEY)")
	cmd.Flags().StringVar(&flags.Endpoint, "llm-endpoint", "", "LLM API endpoint URL")
	cmd.Flags().IntVar(&flags.BatchSize, "batch-size", 8, "Names per LLM request")
	cmd.Flags().IntVar(&flags.SynonymsPerName, "synonyms", defaultSynonyms, "Desired synonyms per name")
	cmd.Flags().StringVar(&flags.SynonymCache, "synonym-cache", ".contexting_synonyms_cache.json", "Path to persistent synonym cache JSON")
	cmd.Flags().StringSliceVar(&flags.ExtraIgnores, "ignore", nil, "Additional ignore entries (name or relative path)")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")

	return cmd
}
