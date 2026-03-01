package main

import (
	"context"
	"fmt"
	"path/filepath"
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
		opts.BatchSize = 8
	}
	if opts.SynonymsPerName <= 0 {
		opts.SynonymsPerName = 4
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

	names := CollectNamesForLLM(tree)
	combined := cloneSynonymMap(opts.SynonymCache)
	missing := missingNames(names, combined)

	var synonymErr error
	if opts.APIKey != "" && len(missing) > 0 {
		generated, err := GenerateSynonymsForNamesWithContext(opts.Ctx, missing, opts.APIKey, opts.BatchSize, opts.Model, opts.SynonymsPerName)
		if err != nil {
			synonymErr = err
		} else {
			for name, values := range generated {
				combined[name] = sanitizeSynonyms(values, opts.SynonymsPerName)
			}
		}
	}

	AssignSynonymsToTree(tree, combined, opts.SynonymsPerName)

	stats := ComputeStats(tree)
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
