package contexting

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newSyncCommand() *cobra.Command {
	flags := CommonFlags{}
	var force bool
	var targetPath string

	cmd := &cobra.Command{
		Use:   "sync [path]",
		Short: "Regenerate synonyms — extracts ESM imports and sends them to the LLM for domain-accurate naming",
		Long: `Regenerate synonyms for code symbols. Extracts ESM imports from each file and sends them to the LLM alongside symbol names, enabling domain-accurate synonym generation.

For example, a file importing "@clerk/nextjs" and "stripe" might get synonyms like "clerk webhook handler" or "payment processing route".

Requires an existing index (run 'ctxt init' first). Missing, short, or stale
symbol/import context is refreshed automatically. Successful responses are saved
when another batch fails; rerun without --force to retry unfinished entries.

Examples:
  ctxt sync .                              Refresh missing or stale synonyms
  ctxt sync . --path command_sync.go        Select one indexed file
  ctxt sync . --path docs --force           Regenerate an indexed directory`,
		Args: cobra.MaximumNArgs(1),
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
			if flags.SymbolExtractor != "" {
				SymbolsExtractorMode = flags.SymbolExtractor
			}
			flags.normalize()

			rootPath := "."
			if len(args) == 1 {
				rootPath = args[0]
			}
			absRoot, err := filepath.Abs(rootPath)
			if err != nil {
				return err
			}
			releaseWriter, err := acquireWriterGuard(absRoot)
			if err != nil {
				return err
			}
			defer releaseWriter()
			outputPath := resolveProjectPath(absRoot, flags.OutputPath)
			cachePath := resolveProjectPath(absRoot, flags.SynonymCache)

			llmEndpoint, llmModel, llmKey, llmTemp, llmMaxTokens, llmProvider := resolveLLMConfig(flags, cfg.LLM)
			LogInfof("LLM: provider=%s model=%s endpoint=%s api_key=%s", llmProvider, llmModel, endpointForLog(llmEndpoint), maskAPIKey(llmKey))
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

			if index.RootPath != absRoot {
				return fmt.Errorf("index root path mismatch: expected %s, got %s", absRoot, index.RootPath)
			}

			// Load synonym cache
			cacheState, err := loadSynonymCacheState(cachePath)
			if err != nil {
				return fmt.Errorf("load synonym cache: %w", err)
			}

			allNames := CollectNamesForLLM(index.Tree)
			if targetPath != "" {
				resolved := resolveProjectPath(absRoot, targetPath)
				if !isInsideProject(resolved, absRoot) {
					return fmt.Errorf("sync path is outside project: %s", targetPath)
				}
				rel, err := filepath.Rel(absRoot, resolved)
				if err != nil {
					return err
				}
				prefix := filepath.ToSlash(rel)
				filtered := make([]string, 0)
				for _, name := range allNames {
					if prefix == "." || name == prefix || strings.HasPrefix(name, prefix+"/") {
						filtered = append(filtered, name)
					}
				}
				allNames = filtered
				if len(allNames) == 0 {
					return fmt.Errorf("sync path is not indexed: %s", targetPath)
				}
			}
			if err := refreshSynonymSymbols(index.Tree, allNames); err != nil {
				return err
			}
			generationContext := collectSynonymContext(index.Tree)
			cache := cacheState.Entries
			synonymsMin, synonymsMax := flags.SynonymsMin, flags.SynonymsMax
			targets := namesNeedingSynonyms(allNames, cache, cacheState.Fingerprints, generationContext.Fingerprints, synonymsMin)
			if force {
				targets = allNames
			}
			if len(targets) == 0 {
				fmt.Println("All selected names have sufficient, current synonyms. Nothing to do.")
				return nil
			}
			fmt.Printf("Generating synonyms for %d missing, short, stale, or explicitly selected names...\n", len(targets))
			ctx, stop := signalAwareContext()
			defer stop()
			batchSize := cfg.Watch.MaxBatchSize
			if cmd.Flags().Changed("batch-size") || batchSize <= 0 {
				batchSize = flags.BatchSize
			}

			generated, err := GenerateSynonymsForNamesWithContext(ctx, targets, llmKey, batchSize, llmModel, llmEndpoint, llmTemp, llmMaxTokens, synonymsMin, synonymsMax, cfg.LLM.ParallelRequests, generationContext.Symbols, generationContext.Imports)
			generationErr := err

			// Update cache with new synonyms
			for name, syns := range generated {
				cache[name] = sanitizeSynonyms(syns, synonymsMax)
				cacheState.Fingerprints[name] = generationContext.Fingerprints[name]
			}

			// Update tree nodes with new synonyms
			AssignSynonymsToTree(index.Tree, cache, synonymsMax)
			index.GeneratedAt = time.Now().UTC()
			index.Model = llmModel

			// Flush updated cache and index
			if err := saveSynonymCacheState(cachePath, cacheState); err != nil {
				return fmt.Errorf("save synonym cache: %w", err)
			}
			if err := SaveContextIndex(outputPath, index); err != nil {
				return fmt.Errorf("save context index: %w", err)
			}

			if generationErr != nil {
				return fmt.Errorf("saved %d successful names; rerun ctxt sync to retry unfinished names: %w", len(generated), generationErr)
			}
			fmt.Printf("Done. Updated %d names. %s flushed.\n", len(generated), filepath.Base(outputPath))
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
	cmd.Flags().StringVar(&flags.SymbolExtractor, "symbol-extractor", "auto", "Symbol extraction engine: auto, treesitter, regex")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")

	cmd.Flags().BoolVar(&force, "force", false, "Regenerate all selected names, including current cache entries")
	cmd.Flags().StringVar(&targetPath, "path", "", "Only sync an indexed file or directory relative to the project root")
	return cmd
}
