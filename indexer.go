package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

type BuildOptions struct {
	Ctx              context.Context
	RootPath         string
	IgnoredPaths     map[string]bool
	APIKey           string
	Model            string
	BatchSize        int
	SynonymsPerName  int
	SynonymCache     SynonymResponse
	MaxBatchSize     int
	Endpoint         string
	Temperature      float64
	MaxTokens        int
	ParallelRequests int // Concurrent LLM requests (default 1 = sequential)
}

type BuildResult struct {
	Index        *ContextIndex
	Stats        IndexStats
	SynonymError error
	SynonymCache SynonymResponse
}

func BuildIndex(opts BuildOptions) (*BuildResult, error) {
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}
	if opts.RootPath == "" {
		opts.RootPath = "."
	}
	if opts.Model == "" {
		opts.Model = defaultModel
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 0 // default: send all
	}
	if opts.SynonymsPerName <= 0 {
		opts.SynonymsPerName = defaultSynonyms
	}
	if opts.IgnoredPaths == nil {
		opts.IgnoredPaths = BuildIgnoreMap(nil)
	}
	if opts.SynonymCache == nil {
		opts.SynonymCache = make(SynonymResponse)
	}

	absRoot, err := filepath.Abs(opts.RootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}

	tree, err := BuildTree(absRoot, opts.IgnoredPaths)
	if err != nil {
		return nil, fmt.Errorf("build tree: %w", err)
	}

	// Stats for progress
	stats := ComputeStats(tree)
	fmt.Printf("✓ Built tree: %d files, %d directories", stats.TotalFiles, stats.TotalDirs)
	if stats.TotalFiles >= MaxFileCount/2 {
		fmt.Printf(" (large repo - consider more ignore patterns)")
	}
	fmt.Println()

	names := CollectNamesForLLM(tree)
	combined := cloneSynonymMap(opts.SynonymCache)
	missing := missingNames(names, combined)

	if len(names) > 0 {
		fmt.Printf("  %d names to process (%d cached, %d new)\n", len(names), len(names)-len(missing), len(missing))
	}

	// Check batch count in main goroutine so interactive prompt works
	runSynonyms := true
	var synonymErr error
	if opts.APIKey != "" && len(missing) > 0 {
		batchSize := opts.MaxBatchSize
		if batchSize <= 0 {
			batchSize = opts.BatchSize
		}
		if batchSize <= 0 {
			if len(missing) <= 60 {
				batchSize = len(missing)
			} else {
				numBatches := (len(missing) + 59) / 60
				batchSize = (len(missing) + numBatches - 1) / numBatches
			}
		}
		totalBatches := (len(missing) + batchSize - 1) / batchSize
		if totalBatches > 9 {
			logWarnf("Project has %d unique names requiring %d batches (>9). This project may be too large for reliable synonym generation.", len(missing), totalBatches)
			if isInteractiveTerminal() {
				continueAnyway, err := askYesNo("Continue anyway? [y/N] ", false)
				if err != nil {
					return nil, fmt.Errorf("prompt failed: %w", err)
				}
				if !continueAnyway {
					runSynonyms = false
					synonymErr = fmt.Errorf("synonym generation aborted: %d batches exceeds threshold", totalBatches)
				}
			}
		}
	}

	// Parallel execution of synonym generation and symbol extraction
	var wg sync.WaitGroup
	var symbolCount int

	// Goroutine A: Synonym generation
	if runSynonyms {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batchSize := opts.MaxBatchSize
			if batchSize <= 0 {
				batchSize = opts.BatchSize
			}
			if opts.APIKey != "" && len(missing) > 0 {
				generated, err := GenerateSynonymsForNamesWithContext(opts.Ctx, missing, opts.APIKey, batchSize, opts.Model, opts.Endpoint, opts.Temperature, opts.MaxTokens, opts.SynonymsPerName, opts.ParallelRequests)
				if err != nil {
					synonymErr = err
				} else {
					for name, values := range generated {
						combined[name] = sanitizeSynonyms(values, opts.SynonymsPerName)
					}
				}
			}
		}()
	}

	// Goroutine B: Symbol extraction
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		walkTree(tree, func(node *Node) {
			if node == tree || node.Type != "file" {
				return
			}
			syms, err := extractSymbols(node.FullPath)
			if err != nil {
				return
			}
			node.Symbols = syms
			count++
			if count%100 == 0 {
				fmt.Printf("\r  Extracting symbols: %d/%d files...", count, stats.TotalFiles)
			}
		})
		symbolCount = count
	}()

	wg.Wait()

	// Assign synonyms after goroutines complete
	AssignSynonymsToTree(tree, combined, opts.SynonymsPerName)

	fmt.Printf("✓ Generated synonyms for %d names\n", len(names))
	fmt.Printf("✓ Extracted symbols from %d files\n", symbolCount)

	stats = ComputeStats(tree)
	stats.CollectedNames = len(names)

	index := &ContextIndex{
		RootPath:    absRoot,
		GeneratedAt: time.Now().UTC(),
		Model:       opts.Model,
		Tree:        tree,
	}

	return &BuildResult{Index: index, Stats: stats, SynonymError: synonymErr, SynonymCache: combined}, nil
}

func missingNames(names []string, cache SynonymResponse) []string {
	missing := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := cache[name]; ok {
			continue
		}
		missing = append(missing, name)
	}
	return missing
}

func cloneSynonymMap(input SynonymResponse) SynonymResponse {
	out := make(SynonymResponse, len(input))
	for name, values := range input {
		out[name] = append([]string(nil), values...)
	}
	return out
}
