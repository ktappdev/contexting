package contexting

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	flags := CommonFlags{}

	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Build context index and write context JSON",
		Long: `Builds the ctxt search index for your project. Walks the directory tree, extracts symbols from source files using tree-sitter (Go/JS/TS/Python/Rust/Svelte/Astro) or regex fallback (Vue/Ruby/C/etc.), and optionally generates LLM synonyms for conceptual search.

Creates two files:
  .ctxt/ctx_index.json    — the searchable index (files, symbols, synonyms, tree)
  .ctxt/ctx_cache.json    — persistent synonym cache (avoids redundant LLM calls)

With an LLM API key, ctxt sends filenames + extracted imports to the LLM to generate domain-accurate synonyms. Without one, falls back to lexical splitting (pascalCase → "pascal case", snake_case → "snake case").

Examples:
  ctxt init .                                    Basic setup
  ctxt init . --model openai/gpt-4o              Use specific LLM model
  ctxt init . --symbol-extractor treesitter      Force tree-sitter (no regex fallback)
  ctxt init . --symbol-extractor regex           Regex-only extraction
  ctxt init . --synonyms 8                       Generate up to 8 synonyms per name
  ctxt init . -v                                 Verbose — see what's being extracted`,
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

			if flags.SymbolExtractor != "" {
				SymbolsExtractorMode = flags.SymbolExtractor
			}

			// If config was just created, pause so user can edit settings before indexing
			if configJustCreated {
				fmt.Println()
				fmt.Printf("Config created at %s. You can customize settings before indexing begins:\n", configPath)
				fmt.Println("  - synonyms: number of synonyms per name (default: 10)")
				fmt.Println("  - batch_size: names per LLM request (default: auto)")
				fmt.Println("  - llm_model: LLM model for synonym generation")
				fmt.Println("  - ignore: paths to exclude from indexing")
				fmt.Println()
				if isInteractiveTerminal() {
					fmt.Print("Edit .ctxt/ctx_config.toml now, then press Enter to continue... ")
					reader := bufio.NewReader(os.Stdin)
					_, _ = reader.ReadString('\n')
				}
				// Re-read config so any edits are picked up
				cfg, err = LoadContextingConfig(absConfigPath)
				if err != nil {
					return err
				}
				applyCommonConfig(cmd, &flags, cfg.Common)
				flags.normalize()
			}

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
			EmbedDotWhitelist(ignored, BuildDotWhitelist(cfg.Common.DotWhitelist))
			// Only skip internal files by basename when the resolved paths are inside the project.
			if isInsideProject(absConfigPath, absRoot) {
				ignored[filepath.Base(absConfigPath)] = true
				ignored[filepath.Base(absConfigPath)+".example"] = true
			}
			if isInsideProject(outputPath, absRoot) {
				ignored[filepath.Base(outputPath)] = true
			}
			llmEndpoint, llmModel, llmKey, llmTemp, llmMaxTokens, llmProvider := resolveLLMConfig(flags, cfg.LLM)
			LogInfof("LLM: provider=%s model=%s endpoint=%s api_key=%s", llmProvider, llmModel, llmEndpoint, maskAPIKey(llmKey))
			cache, err := LoadSynonymCache(cachePath)
			if err != nil {
				return err
			}
			if llmKey == "" {
				LogWarnf("LLM API key not configured; continuing without synonyms")
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
				SynonymsMin:      flags.SynonymsMin,
				SynonymsMax:      flags.SynonymsMax,
				SynonymCache:     cache,
				MaxBatchSize:     cfg.Watch.MaxBatchSize,
				Endpoint:         llmEndpoint,
				Temperature:      llmTemp,
				MaxTokens:        llmMaxTokens,
				ParallelRequests: cfg.LLM.ParallelRequests,
				Verbose:          flags.Verbose,
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

			LogInfof("Indexed %d nodes (%d files, %d directories). Synonyms on %d nodes.", result.Stats.TotalNodes, result.Stats.TotalFiles, result.Stats.TotalDirs, result.Stats.SynonymNodes)
			LogInfof("Collected %d unique names. Wrote %s", result.Stats.CollectedNames, outputPath)
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
	cmd.Flags().StringSliceVar(&flags.ExtraIgnores, "ignore", nil, "Additional ignore entries (name or relative path)")
	cmd.Flags().StringVar(&flags.SymbolExtractor, "symbol-extractor", "auto", "Symbol extraction engine: auto, treesitter, regex")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")

	return cmd
}
