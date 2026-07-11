package contexting

import (
	"context"
	"fmt"
	"path/filepath"
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
	SynonymsMin      int
	SynonymsMax      int
	SynonymCache     SynonymResponse
	MaxBatchSize     int
	Endpoint         string
	Temperature      float64
	MaxTokens        int
	ParallelRequests int // Concurrent LLM requests (default 1 = sequential)
	Verbose          bool   // Enable verbose progress output
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
	if opts.SynonymsMin <= 0 {
		opts.SynonymsMin = opts.SynonymsPerName
	}
	if opts.SynonymsMax <= 0 {
		opts.SynonymsMax = defaultSynonymsMax
	}
	if opts.SynonymsMin > opts.SynonymsMax {
		opts.SynonymsMin = opts.SynonymsMax
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
	extraMsg := ""
	if stats.TotalFiles >= MaxFileCount/2 {
		extraMsg = " (large repo - consider more ignore patterns)"
	}
	LogInfof("✓ Built tree: %d files, %d directories%s", stats.TotalFiles, stats.TotalDirs, extraMsg)

	names := CollectNamesForLLM(tree)
	combined := cloneSynonymMap(opts.SynonymCache)
	missing := missingNames(names, combined)

	if len(names) > 0 {
		LogInfof("  %d names to process (%d cached, %d new)", len(names), len(names)-len(missing), len(missing))
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
			LogWarnf("Project has %d unique names requiring %d batches (>9). This project may be too large for reliable synonym generation.", len(missing), totalBatches)
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

	// Sequential execution: extract symbols FIRST, then generate synonyms with symbols context
	var symbolCount int

	// Step 1: Extract symbols (local, fast)
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
		if count%100 == 0 && opts.Verbose {
			LogInfof("  Extracting symbols: %d/%d files", count, stats.TotalFiles)
		}
	})
	symbolCount = count

	// Step 2: Build symbols map for LLM context
	symbolsMap := make(map[string][]string)
	walkTree(tree, func(node *Node) {
		if node.Type == "file" && len(node.Symbols) > 0 {
			name := filepath.Base(node.FullPath)
			symbolsMap[name] = node.Symbols
		}
	})

	// Step 3: Generate synonyms with symbols context
	if runSynonyms && opts.APIKey != "" && len(missing) > 0 {
		batchSize := opts.MaxBatchSize
		if batchSize <= 0 {
			batchSize = opts.BatchSize
		}
		generated, err := GenerateSynonymsForNamesWithContext(opts.Ctx, missing, opts.APIKey, batchSize, opts.Model, opts.Endpoint, opts.Temperature, opts.MaxTokens, opts.SynonymsMin, opts.SynonymsMax, opts.ParallelRequests, symbolsMap)
		if err != nil {
			synonymErr = err
		} else {
			for name, values := range generated {
				combined[name] = sanitizeSynonyms(values, opts.SynonymsMax)
			}
		}
	}

	// Assign synonyms after goroutines complete
	AssignSynonymsToTree(tree, combined, opts.SynonymsMax)

	LogInfof("✓ Generated synonyms for %d names", len(names))
	LogInfof("✓ Extracted symbols from %d files", symbolCount)

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
