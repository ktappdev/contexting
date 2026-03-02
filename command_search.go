package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newSearchCommand() *cobra.Command {
	var rootPath string
	var indexPath string
	var runtimeFile string
	var opts SearchOptions
	var dirSummary bool
	var dirLimit int
	var drillLimit int
	var jsonOut bool
	var showTokens bool
	var useMemory bool
	var memoryOnly bool

	cmd := &cobra.Command{
		Use:   "search-hints [query]",
		Short: "Find top matching paths from context JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadContextingConfig(configPath)
			if err != nil {
				return err
			}
			if rootPath == "" {
				rootPath, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("get working directory: %w", err)
				}
			}
			absRoot, err := filepath.Abs(rootPath)
			if err != nil {
				return fmt.Errorf("resolve root path: %w", err)
			}
			applyStringFlag(cmd, "index", &indexPath, cfg.Search.IndexPath)
			applyIntFlag(cmd, "limit", &opts.Limit, cfg.Search.Limit)
			applyIntFlag(cmd, "min-score", &opts.MinScore, cfg.Search.MinScore)
			applyStringFlag(cmd, "type", &opts.TypeFilter, cfg.Search.TypeFilter)
			if cfg.Search.DirSummary != nil {
				applyBoolFlag(cmd, "dir-summary", &dirSummary, *cfg.Search.DirSummary)
			}
			applyIntFlag(cmd, "dir-limit", &dirLimit, cfg.Search.DirLimit)
			applyIntFlag(cmd, "drill-limit", &drillLimit, cfg.Search.DrillLimit)
			if cfg.Search.Explain != nil {
				applyBoolFlag(cmd, "explain", &opts.IncludeDebug, *cfg.Search.Explain)
			}
			if cfg.Search.JSON != nil {
				applyBoolFlag(cmd, "json", &jsonOut, *cfg.Search.JSON)
			}
			if cfg.Search.ShowTokens != nil {
				applyBoolFlag(cmd, "show-tokens", &showTokens, *cfg.Search.ShowTokens)
			}
			if cfg.Search.UseMemory != nil {
				applyBoolFlag(cmd, "memory", &useMemory, *cfg.Search.UseMemory)
			}
			applyStringFlag(cmd, "runtime-file", &runtimeFile, cfg.Search.RuntimeFile)
			if !cmd.Flags().Changed("index") {
				indexPath = resolveConfigPath(configPath, indexPath)
			}
			if runtimeFile == "" {
				runtimeFile = resolveProjectPath(filepath.Dir(indexPath), ".contexting_runtime.json")
			} else if !cmd.Flags().Changed("runtime-file") {
				runtimeFile = resolveConfigPath(configPath, runtimeFile)
			}

			query := args[0]
			results := make([]SearchResult, 0)
			usedMemory := false
			if useMemory {
				memoryResults, memErr := QueryMemorySearch(runtimeFile, query, opts, absRoot)
				if memErr == nil {
					results = memoryResults
					usedMemory = true
				} else if memoryOnly {
					return memErr
				} else {
					logWarnf("Memory search unavailable, falling back to snapshot index: %v", memErr)
				}
			}
			if !usedMemory {
				index, err := LoadContextIndex(indexPath)
				if err != nil {
					return err
				}
				if index.RootPath == "" {
					return fmt.Errorf("index missing root_path: regenerate index by running 'contexting watch' or 'contexting init' in the project directory")
				}
				if index.RootPath != absRoot {
					return fmt.Errorf("index root path mismatch: expected %s, got %s. Use --root to specify the project directory or run from the project root", absRoot, index.RootPath)
				}
				results = SearchHintsWithOptions(index, query, opts)
			}
			if showTokens {
				fmt.Printf("Tokens: %v\n", tokenize(query))
			}

			if dirSummary {
				summaries := SummarizeDirectories(results, dirLimit, drillLimit)
				if jsonOut {
					jsonStr, err := directorySummariesToJSON(summaries)
					if err != nil {
						return err
					}
					fmt.Println(jsonStr)
					return nil
				}
				printDirectorySummaries(summaries)
				return nil
			}

			if jsonOut {
				jsonStr, err := resultsToJSON(results)
				if err != nil {
					return err
				}
				fmt.Println(jsonStr)
				return nil
			}

			printSearchResults(results)
			return nil
		},
	}

	cmd.Flags().StringVar(&rootPath, "root", "", "Project root path (defaults to current working directory)")
	cmd.Flags().StringVarP(&indexPath, "index", "i", "context.json", "Path to context JSON")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "n", 5, "Maximum number of matches")
	cmd.Flags().IntVar(&opts.MinScore, "min-score", 1, "Minimum score required to return a match")
	cmd.Flags().StringVar(&opts.TypeFilter, "type", "all", "Filter result type: all|files|dirs")
	cmd.Flags().BoolVar(&dirSummary, "dir-summary", false, "Summarize top matching directories with rationale and drill-down hits")
	cmd.Flags().IntVar(&dirLimit, "dir-limit", 5, "Maximum number of directories returned in --dir-summary mode")
	cmd.Flags().IntVar(&drillLimit, "drill-limit", 3, "Maximum top hits shown per directory in --dir-summary mode")
	cmd.Flags().BoolVar(&opts.IncludeDebug, "explain", false, "Include score breakdown in output")
	cmd.Flags().BoolVar(&useMemory, "memory", true, "Query live in-memory watch index when available")
	cmd.Flags().BoolVar(&memoryOnly, "memory-only", false, "Require live memory search and fail instead of falling back to snapshot")
	cmd.Flags().StringVar(&runtimeFile, "runtime-file", "", "Path to runtime memory-search state file (defaults near index path)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print search results as JSON")
	cmd.Flags().BoolVar(&showTokens, "show-tokens", false, "Print normalized query tokens before results")

	return cmd
}
