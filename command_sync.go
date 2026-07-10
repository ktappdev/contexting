package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newSyncCommand() *cobra.Command {
	flags := CommonFlags{}

	cmd := &cobra.Command{
		Use:   "sync [path]",
		Short: "Generate synonyms for names that are missing or short",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var absConfigPath string
			if configPath != "" {
				var err error
				absConfigPath, err = filepath.Abs(configPath)
				if err != nil {
					return fmt.Errorf("resolve config path: %w", err)
				}
			}
			cfg, err := LoadContextingConfig(absConfigPath)
			if err != nil {
				return err
			}
			applyCommonConfig(cmd, &flags, cfg.Common)
			flags.normalize()

			rootPath := "."
			if len(args) == 1 {
				rootPath = args[0]
			}
			absRoot, err := filepath.Abs(rootPath)
			if err != nil {
				return err
			}
			outputPath := resolveProjectPath(absRoot, flags.OutputPath)
			cachePath := resolveProjectPath(absRoot, flags.SynonymCache)

			llmEndpoint, llmModel, llmKey, llmTemp, llmMaxTokens, llmProvider := resolveLLMConfig(flags, cfg.LLM)
			logInfof("LLM: provider=%s model=%s endpoint=%s api_key=%s", llmProvider, llmModel, llmEndpoint, maskAPIKey(llmKey))
			if llmKey == "" {
				return fmt.Errorf("LLM API key not configured; cannot generate synonyms")
			}

			// Load existing index
			index, err := LoadContextIndex(outputPath)
			if err != nil {
				return fmt.Errorf("load context index: %w", err)
			}
			if index == nil || index.Tree == nil {
				return fmt.Errorf("no context index found at %s; run 'ctxt init' first", outputPath)
			}

			// Load synonym cache
			cache, err := LoadSynonymCache(cachePath)
			if err != nil {
				return fmt.Errorf("load synonym cache: %w", err)
			}

			// Collect all names from tree
			allNames := CollectNamesForLLM(index.Tree)
			fmt.Printf("Scanning index... %d names found\n", len(allNames))

			// Determine which names need synonyms
			synonymsMin := flags.SynonymsMin
			synonymsMax := flags.SynonymsMax
			var needsSynonyms []string
			var needsShort []string
			for _, name := range allNames {
				cached, ok := cache[name]
				if !ok {
					needsSynonyms = append(needsSynonyms, name)
				} else if len(cached) < synonymsMin {
					needsShort = append(needsShort, name)
				}
			}

			targets := append(needsSynonyms, needsShort...)
			if len(targets) == 0 {
				fmt.Printf("All names have sufficient synonyms. Nothing to do.\n")
				return nil
			}

			fmt.Printf("%d names need synonyms (%d missing, %d short)\n", len(targets), len(needsSynonyms), len(needsShort))
			fmt.Printf("Generating synonyms for %d names...\n", len(targets))

			// Generate synonyms for targets
			ctx, stop := signalAwareContext()
			defer stop()

			// Build symbols map for targets
			symbolsMap := make(map[string][]string)
			walkTree(index.Tree, func(node *Node) {
				if node.Type != "file" {
					return
				}
				name := filepath.Base(node.FullPath)
				if len(node.Symbols) > 0 {
					symbolsMap[name] = node.Symbols
				} else {
					// Extract symbols if missing
					if syms, err := extractSymbols(node.FullPath); err == nil && len(syms) > 0 {
						node.Symbols = syms
						symbolsMap[name] = syms
					}
				}
			})

			generated, err := GenerateSynonymsForNamesWithContext(ctx, targets, llmKey, cfg.Watch.MaxBatchSize, llmModel, llmEndpoint, llmTemp, llmMaxTokens, synonymsMin, synonymsMax, cfg.LLM.ParallelRequests, symbolsMap)
			if err != nil {
				return fmt.Errorf("generate synonyms: %w", err)
			}

			// Update cache with new synonyms
			for name, syns := range generated {
				cache[name] = sanitizeSynonyms(syns, synonymsMax)
			}

			// Update tree nodes with new synonyms
			AssignSynonymsToTree(index.Tree, cache, synonymsMax)

			// Flush updated cache and index
			if err := SaveSynonymCache(cachePath, cache); err != nil {
				return fmt.Errorf("save synonym cache: %w", err)
			}
			if err := SaveContextIndex(outputPath, index); err != nil {
				return fmt.Errorf("save context index: %w", err)
			}

			fmt.Printf("Done. Updated %d names. %s flushed.\n", len(targets), filepath.Base(outputPath))
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.OutputPath, "output", "o", ".ctxt/ctx_index.json", "Output JSON path")
	cmd.Flags().StringVar(&flags.Model, "llm-model", "", "LLM model used for synonym generation")
	cmd.Flags().StringVar(&flags.APIKey, "api-key", "", "LLM API key (falls back to config api_key_env, LLM_API_KEY, OPENROUTER_API_KEY)")
	cmd.Flags().StringVar(&flags.Endpoint, "llm-endpoint", "", "LLM API endpoint URL")
	cmd.Flags().IntVar(&flags.BatchSize, "batch-size", 8, "Names per LLM request")
	cmd.Flags().IntVar(&flags.SynonymsPerName, "synonyms", defaultSynonyms, "Synonyms per name (fallback for min/max)")
	cmd.Flags().IntVar(&flags.SynonymsMin, "synonyms-min", 0, "Min synonyms per name (0 = use synonyms value)")
	cmd.Flags().IntVar(&flags.SynonymsMax, "synonyms-max", 0, "Max synonyms per name (0 = use synonyms value)")
	cmd.Flags().StringVar(&flags.SynonymCache, "synonym-cache", ".ctxt/ctx_cache.json", "Path to persistent synonym cache JSON")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")

	return cmd
}
