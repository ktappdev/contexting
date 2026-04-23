package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

type BuildOptions struct {
	Ctx             context.Context
	RootPath        string
	IgnoredPaths    map[string]bool
	APIKey          string
	Model           string
	BatchSize       int
	SynonymsPerName int
	SynonymCache    SynonymResponse
	MaxBatchSize    int
	Endpoint        string
	Temperature     float64
	MaxTokens       int
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

	// Parallel execution of synonym generation and symbol extraction
	var wg sync.WaitGroup
	var synonymErr error
	var symbolCount int

	wg.Add(2)

	// Goroutine A: Synonym generation
	go func() {
		defer wg.Done()
		batchSize := opts.MaxBatchSize
		if batchSize <= 0 {
			batchSize = opts.BatchSize
		}
		if opts.APIKey != "" && len(missing) > 0 {
			generated, err := GenerateSynonymsForNamesWithContext(opts.Ctx, missing, opts.APIKey, batchSize, opts.Model, opts.Endpoint, opts.Temperature, opts.MaxTokens, opts.SynonymsPerName)
			if err != nil {
				synonymErr = err
			} else {
				for name, values := range generated {
					combined[name] = sanitizeSynonyms(values, opts.SynonymsPerName)
				}
			}
		}
	}()

	// Goroutine B: Symbol extraction
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
