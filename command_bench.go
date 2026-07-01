package main

import (
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

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Benchmark search engines against a query case set",
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
			applyIntFlag(cmd, "limit", &opts.Limit, cfg.Bench.Limit)
			applyIntFlag(cmd, "min-score", &opts.MinScore, cfg.Bench.MinScore)
			applyStringFlag(cmd, "engines", &engines, strings.Join(cfg.Bench.Engines, ","))
			applyIntFlag(cmd, "grep-max-bytes", &grepMaxBytes, cfg.Bench.GrepMaxBytes)
			if cfg.Bench.JSON != nil {
				applyBoolFlag(cmd, "json", &jsonOut, *cfg.Bench.JSON)
			}
			if grepMaxBytes <= 0 {
				grepMaxBytes = 1048576
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

			cases, err := LoadCasesAuto(casesPath)
			if err != nil {
				return err
			}

			engineNames := strings.Split(engines, ",")
			for i, name := range engineNames {
				engineNames[i] = strings.TrimSpace(strings.ToLower(name))
			}
			for _, name := range engineNames {
				if !isKnownEngine(name) {
					return fmt.Errorf("unknown engine %q (known engines: %s)", name, strings.Join(knownEngines(), ", "))
				}
			}
			engineList := instantiateEngines(engineNames)

			var index *ContextIndex
			needContexting := false
			for _, name := range engineNames {
				if name == "contexting" {
					needContexting = true
					break
				}
			}
			indexStart := time.Now()
			index, err = LoadContextIndex(indexPath)
			indexLoadMs := time.Since(indexStart).Milliseconds()
			if err != nil {
				if needContexting {
					return err
				}
				logWarnf("index load failed, contexting engine will be skipped: %v", err)
				indexLoadMs = 0
				index = &ContextIndex{RootPath: absRoot}
			} else {
				if index.RootPath == "" {
					return fmt.Errorf("index missing root_path: regenerate index by running 'contexting watch' or 'contexting init' in the project directory")
				}
				if index.RootPath != absRoot {
					return fmt.Errorf("index root path mismatch: expected %s, got %s. Use --root to specify the project directory or run from the project root", absRoot, index.RootPath)
				}
			}
			if index == nil {
				for i, name := range engineNames {
					if name == "contexting" {
						engineNames = append(engineNames[:i], engineNames[i+1:]...)
						break
					}
				}
				engineList = instantiateEngines(engineNames)
				index = &ContextIndex{RootPath: absRoot}
				indexLoadMs = 0
			}

			out := runBench(BenchInput{
				Index:        index,
				Cases:        cases,
				Engines:      engineList,
				Limit:        opts.Limit,
				MinScore:     opts.MinScore,
				GrepMaxBytes: grepMaxBytes,
			})
			out.IndexLoadMs = indexLoadMs

			// Check if cases have categories
			hasCategories := false
			if len(cases) > 0 && cases[0].Category != "" {
				hasCategories = true
			}

			if jsonOut {
				if byCategory && hasCategories {
					jsonStr, err := categoryReportToJSON(cases, out.Results, out.IndexLoadMs)
					if err != nil {
						return err
					}
					fmt.Println(jsonStr)
				} else {
					jsonStr, err := resultsToJSONBench(out)
					if err != nil {
						return err
					}
					fmt.Println(jsonStr)
				}
				return nil
			}

			if out.IndexLoadMs >= 0 {
				fmt.Printf("Index load: %dms (one-time cost)\n\n", out.IndexLoadMs)
			}
			if byCategory && hasCategories {
				printCategoryReport(cases, out.Results)
			} else {
				printBenchSummary(out)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&rootPath, "root", "", "Project root path (defaults to current working directory)")
	cmd.Flags().StringVarP(&indexPath, "index", "i", ".ctx/ctx_index.json", "Path to context JSON")
	cmd.Flags().StringVarP(&casesPath, "cases", "c", "", "Path to eval cases JSON")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "n", 10, "Number of ranked search results per query")
	cmd.Flags().IntVar(&opts.MinScore, "min-score", 1, "Minimum score required for candidate results")
	cmd.Flags().StringVar(&engines, "engines", "contexting,find,grep", "Comma-separated list of engines to benchmark")
	cmd.Flags().IntVar(&grepMaxBytes, "grep-max-bytes", 1048576, "Maximum file size in bytes for grep engine to read")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print full benchmark report as JSON")
	cmd.Flags().BoolVar(&byCategory, "by-category", true, "Group report by category (for v2 case files)")

	return cmd
}

// BenchInput is the testable input to runBench.
type BenchInput struct {
	Index        *ContextIndex
	Cases        []EvalCase
	Engines      []SearchEngine
	Limit        int
	MinScore     int
	GrepMaxBytes int
}

// runBench executes every engine on every case and returns the full report.
func runBench(in BenchInput) BenchOutput {
	out := BenchOutput{
		Cases:   in.Cases,
		Results: make([][]EngineResult, 0, len(in.Cases)),
	}
	if len(in.Cases) == 0 {
		return out
	}

	searchOpts := SearchOptions{
		Limit:    in.Limit,
		MinScore: in.MinScore,
	}
	if searchOpts.Limit <= 0 {
		searchOpts.Limit = 10
	}
	if searchOpts.MinScore < 0 {
		searchOpts.MinScore = 1
	}
	if searchOpts.TypeFilter == "" {
		searchOpts.TypeFilter = "all"
	}

	for _, c := range in.Cases {
		perEngine := make([]EngineResult, 0, len(in.Engines))
		for _, engine := range in.Engines {
			res := engine.Search(c.Query, c.ExpectAny, in.Index, searchOpts, in.GrepMaxBytes)
			perEngine = append(perEngine, res)
		}
		out.Results = append(out.Results, perEngine)
	}

	out.Summary = computeEngineSummaries(in.Cases, out.Results)
	out.Misses = computeBenchMisses(in.Cases, out.Results)
	return out
}
