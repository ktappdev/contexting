package contexting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newBenchCommand() *cobra.Command {
	var rootPath string
	var indexPath string
	var casesPath string
	var engines string
	var opts SearchOptions
	var jsonOut bool
	var grepMaxBytes int
	var byCategory bool
	var runs, warmup int

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Benchmark search engines against a query case set",
		Long: `Benchmarks ctxt and other search engines against a set of query cases to measure search quality. Each case defines a query, expected file paths, and an optional intent category.

Metrics reported:
  Hit@1 — fraction of queries where the first result is an expected file
  Hit@3/Hit@5 — fraction where expected file appears in top 3/5 results
  Success — fraction of queries finding any expected file within --limit
  Noise — fraction of results that aren't expected files
  Path tokens — path bytes / 4 only, excluding response metadata and content

7 engines available: ctxt, find, fd, grep, rg, hybrid, combined (default: ctxt,find,grep).

All engines search indexed files and share --limit. Expected paths must match exactly.
ctxt/hybrid order by relevance; other engines order alphabetically.
Timings repeat adapter calls with rotated engine order; subprocess startup is included for external tools.
Use 'ctxt eval' for per-query debugging; use 'ctxt bench' for aggregate comparison.

Examples:
  ctxt bench --cases docs/bench_cases.json                    Run with default engines
  ctxt bench --cases bench.json --engines ctxt,rg,hybrid      Compare specific engines
  ctxt bench --cases cases.json --json > results.json        JSON export
  ctxt bench --cases cases.json --limit 20                    More results per query`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var absConfigPath string
			if configPath != "" {
				var cfgErr error
				absConfigPath, cfgErr = filepath.Abs(configPath)
				if cfgErr != nil {
					return fmt.Errorf("resolve config path: %w", cfgErr)
				}
			}
			cfg, err := LoadContextingConfig(absConfigPath)
			if err != nil {
				return err
			}
			if rootPath == "" {
				if cfg.Bench.RootPath != "" {
					rootPath = cfg.Bench.RootPath
				} else {
					rootPath, err = os.Getwd()
					if err != nil {
						return fmt.Errorf("get working directory: %w", err)
					}
				}
			}
			absRoot, err := filepath.Abs(rootPath)
			if err != nil {
				return fmt.Errorf("resolve root path: %w", err)
			}
			applyStringFlag(cmd, "index", &indexPath, cfg.Bench.IndexPath)
			applyStringFlag(cmd, "cases", &casesPath, cfg.Bench.CasesPath)
			if cfg.Bench.Limit != nil {
				applyIntFlag(cmd, "limit", &opts.Limit, *cfg.Bench.Limit)
			}
			if cfg.Bench.MinScore != nil {
				applyIntFlag(cmd, "min-score", &opts.MinScore, *cfg.Bench.MinScore)
			}
			applyStringFlag(cmd, "engines", &engines, strings.Join(cfg.Bench.Engines, ","))
			if cfg.Bench.GrepMaxBytes != nil {
				applyIntFlag(cmd, "grep-max-bytes", &grepMaxBytes, *cfg.Bench.GrepMaxBytes)
			}
			if cfg.Bench.Runs != nil {
				applyIntFlag(cmd, "runs", &runs, *cfg.Bench.Runs)
			}
			if cfg.Bench.Warmup != nil {
				applyIntFlag(cmd, "warmup", &warmup, *cfg.Bench.Warmup)
			}
			if runs < 1 || warmup < 0 {
				return fmt.Errorf("--runs must be positive and --warmup must be nonnegative")
			}
			if opts.Limit < 1 {
				return fmt.Errorf("--limit must be positive")
			}
			if cfg.Bench.JSON != nil {
				applyBoolFlag(cmd, "json", &jsonOut, *cfg.Bench.JSON)
			}
			if grepMaxBytes <= 0 {
				return fmt.Errorf("--grep-max-bytes must be positive")
			}
			if opts.MinScore < 0 {
				return fmt.Errorf("--min-score must be nonnegative")
			}
			if !cmd.Flags().Changed("index") {
				indexPath = resolveConfigPath(absConfigPath, indexPath)
			}
			if !cmd.Flags().Changed("cases") {
				casesPath = resolveConfigPath(absConfigPath, casesPath)
			}

			if casesPath == "" {
				return fmt.Errorf("--cases is required")
			}

			cases, err := LoadEvalCases(casesPath)
			if err != nil {
				return err
			}

			engineNames := strings.Split(engines, ",")
			for i, name := range engineNames {
				engineNames[i] = strings.TrimSpace(strings.ToLower(name))
			}
			seenEngines := make(map[string]bool)
			for _, name := range engineNames {
				if seenEngines[name] {
					return fmt.Errorf("duplicate benchmark engine %q", name)
				}
				seenEngines[name] = true
				if !isKnownEngine(name) {
					return fmt.Errorf("unknown engine %q (known engines: %s)", name, strings.Join(knownEngines(), ", "))
				}
			}
			engineList := instantiateEngines(engineNames)

			var index *ContextIndex
			needCtxt := false
			for _, name := range engineNames {
				if name == "ctxt" || name == "hybrid" {
					needCtxt = true
					break
				}
			}
			indexPath = resolveProjectPath(absRoot, indexPath)
			indexStart := time.Now()
			index, err = LoadContextIndex(indexPath)
			if err != nil {
				if needCtxt || !errors.Is(err, os.ErrNotExist) {
					return err
				}
				// Baseline-only runs can build a file list without LLM calls or writes.
				ignored := BuildIgnoreMap(cfg.Common.ExtraIgnores)
				EmbedDotWhitelist(ignored, BuildDotWhitelist(cfg.Common.DotWhitelist))
				patterns, ignoreErr := LoadGitignorePatterns(filepath.Join(absRoot, ".gitignore"))
				if ignoreErr != nil && !errors.Is(ignoreErr, os.ErrNotExist) {
					return ignoreErr
				}
				ignored[gitignoreRulesKey+strings.Join(patterns, "\n")] = true
				tree, buildErr := BuildTree(absRoot, ignored)
				if buildErr != nil {
					return buildErr
				}
				index = &ContextIndex{RootPath: absRoot, Tree: tree}
			}
			indexLoadMs := float64(time.Since(indexStart)) / float64(time.Millisecond)
			expectedRoot, rootErr := filepath.EvalSymlinks(absRoot)
			indexedRoot, indexRootErr := filepath.EvalSymlinks(index.RootPath)
			if rootErr != nil || indexRootErr != nil || expectedRoot != indexedRoot {
				return fmt.Errorf("index root path mismatch: expected %s, got %s; rebuild with ctxt init", absRoot, index.RootPath)
			}
			if err := validateBenchCases(cases, index); err != nil {
				return err
			}

			out := runBench(BenchInput{
				Index:        index,
				Cases:        cases,
				Engines:      engineList,
				Limit:        opts.Limit,
				MinScore:     opts.MinScore,
				GrepMaxBytes: grepMaxBytes,
				Runs:         runs,
				Warmup:       warmup,
			})
			out.IndexLoadMs = indexLoadMs

			// Check if cases have categories
			hasCategories := len(cases) > 0 && cases[0].Category != ""

			if !byCategory {
				out.Categories = nil
			}
			if jsonOut {
				jsonStr, err := resultsToJSONBench(out)
				if err != nil {
					return err
				}
				fmt.Println(jsonStr)
				if benchHasErrors(out) {
					return fmt.Errorf("benchmark encountered engine execution errors; see report")
				}
				return nil
			}

			if out.IndexLoadMs >= 0 {
				fmt.Printf("Index/file-list load: %.3fms (one-time cost)\n", out.IndexLoadMs)
			}
			if needCtxt {
				if index.Model != "" {
					fmt.Printf("Index model: %s\n", index.Model)
				} else {
					fmt.Println("Note: No LLM synonyms in index (lexical only)")
				}
				fmt.Println()
			}
			fmt.Printf("Shared corpus preparation: %.3fms (one-time cost)\n", out.CorpusPreparationMs)
			fmt.Printf("Timing: %d measured runs, %d warmups per query; limit %d files per engine\n", runs, warmup, opts.Limit)
			if byCategory && hasCategories {
				printCategoryReport(cases, out.Results)
			} else {
				printBenchSummary(out)
			}
			if benchHasErrors(out) {
				return fmt.Errorf("benchmark encountered engine execution errors; see report")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&rootPath, "root", "", "Project root path (defaults to current working directory)")
	cmd.Flags().StringVarP(&indexPath, "index", "i", ".ctxt/ctx_index.json", "Path to context JSON")
	cmd.Flags().StringVarP(&casesPath, "cases", "c", "", "Path to eval cases JSON")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "n", 10, "Maximum files returned per query by every engine")
	cmd.Flags().IntVar(&opts.MinScore, "min-score", 1, "Minimum index score for ctxt/hybrid results")
	cmd.Flags().StringVar(&engines, "engines", "ctxt,find,grep", "Engines to benchmark: ctxt,find,fd,grep,rg,hybrid,combined")
	cmd.Flags().IntVar(&grepMaxBytes, "grep-max-bytes", 1048576, "Max file size in bytes for grep, rg, and hybrid content search")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print full benchmark report as JSON")
	cmd.Flags().BoolVar(&byCategory, "by-category", true, "Group report by category (for v2 case files)")

	cmd.Flags().IntVar(&runs, "runs", 5, "Measured runs per query and engine")
	cmd.Flags().IntVar(&warmup, "warmup", 1, "Unmeasured warmups per query and engine")
	return cmd
}
