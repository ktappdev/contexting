package contexting

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"
)

type ctxtEngine struct{}

func (ctxtEngine) Name() string { return "ctxt" }
func (ctxtEngine) Search(query string, expected []string, index *ContextIndex, opts SearchOptions, _ int, corpus BenchCorpus) EngineResult {
	start := time.Now()
	opts.TypeFilter = "files"
	results := SearchHintsWithOptions(index, query, opts)
	paths := make([]string, 0, len(results))
	for _, result := range results {
		paths = append(paths, result.Path)
	}
	return benchResult("ctxt", paths, expected, opts.Limit, start, nil)
}

type hybridEngine struct{}

func (hybridEngine) Name() string { return "hybrid" }
func (hybridEngine) Search(query string, expected []string, index *ContextIndex, opts SearchOptions, maxBytes int, corpus BenchCorpus) EngineResult {
	start := time.Now()
	unlimited := opts
	unlimited.Limit = int(^uint(0) >> 1)
	unlimited.TypeFilter = "files"
	unlimited.ContentFallback = false
	results := SearchHintsWithOptions(index, query, unlimited)
	if opts.MinScore <= 1 {
		contentOpts := opts
		contentOpts.Limit = -1
		content := benchContentSearch("rg", query, expected, index, contentOpts, maxBytes, corpus)
		if content.Error != "" {
			return benchResult("hybrid", nil, expected, opts.Limit, start, fmt.Errorf("content fallback: %s", content.Error))
		}
		existing := make(map[string]bool)
		for _, result := range results {
			existing[filepath.ToSlash(result.Path)] = true
		}
		for _, path := range content.Paths {
			if !existing[path] {
				results = append(results, SearchResult{Path: path, Type: "file", Score: 1})
			}
		}
		sort.Slice(results, func(i, j int) bool {
			if results[i].Score != results[j].Score {
				return results[i].Score > results[j].Score
			}
			return results[i].Path < results[j].Path
		})
	}
	paths := make([]string, 0, len(results))
	for _, result := range results {
		paths = append(paths, filepath.ToSlash(result.Path))
	}
	return benchResult("hybrid", paths, expected, opts.Limit, start, nil)
}

type unixFindEngine struct{}

func (unixFindEngine) Name() string { return "find" }
func (unixFindEngine) Search(query string, expected []string, index *ContextIndex, opts SearchOptions, _ int, corpus BenchCorpus) EngineResult {
	return benchFilenameSearch("find", query, expected, index, opts, corpus)
}

type fdEngine struct{}

func (fdEngine) Name() string { return "fd" }
func (fdEngine) Search(query string, expected []string, index *ContextIndex, opts SearchOptions, _ int, corpus BenchCorpus) EngineResult {
	return benchFilenameSearch("fd", query, expected, index, opts, corpus)
}

type grepEngine struct{}

func (grepEngine) Name() string { return "grep" }
func (grepEngine) Search(query string, expected []string, index *ContextIndex, opts SearchOptions, maxBytes int, corpus BenchCorpus) EngineResult {
	return benchContentSearch("grep", query, expected, index, opts, maxBytes, corpus)
}

type rgEngine struct{}

func (rgEngine) Name() string { return "rg" }
func (rgEngine) Search(query string, expected []string, index *ContextIndex, opts SearchOptions, maxBytes int, corpus BenchCorpus) EngineResult {
	return benchContentSearch("rg", query, expected, index, opts, maxBytes, corpus)
}

type combinedEngine struct {
	find unixFindEngine
	grep grepEngine
}

func (combinedEngine) Name() string { return "combined" }
func (engine combinedEngine) Search(query string, expected []string, index *ContextIndex, opts SearchOptions, maxBytes int, corpus BenchCorpus) EngineResult {
	start := time.Now()
	// Union full candidate lists, then apply the shared output budget.
	unlimited := opts
	unlimited.Limit = -1
	filename := engine.find.Search(query, expected, index, unlimited, maxBytes, corpus)
	content := engine.grep.Search(query, expected, index, unlimited, maxBytes, corpus)
	if filename.Error != "" || content.Error != "" {
		return benchResult("combined", nil, expected, opts.Limit, start, fmt.Errorf("find: %s; grep: %s", filename.Error, content.Error))
	}
	seen := make(map[string]bool)
	paths := make([]string, 0, len(filename.Paths)+len(content.Paths))
	for _, path := range append(filename.Paths, content.Paths...) {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return benchResult("combined", paths, expected, opts.Limit, start, nil)
}

// knownEngines returns the list of supported engine names in default order.
func knownEngines() []string {
	return []string{"ctxt", "find", "grep", "combined", "fd", "rg", "hybrid"}
}

// isKnownEngine reports whether name is a supported engine.
func isKnownEngine(name string) bool {
	for _, e := range knownEngines() {
		if e == name {
			return true
		}
	}
	return false
}

// instantiateEngines creates SearchEngine implementations from names.
func instantiateEngines(names []string) []SearchEngine {
	out := make([]SearchEngine, 0, len(names))
	for _, name := range names {
		switch name {
		case "ctxt":
			out = append(out, ctxtEngine{})
		case "find":
			out = append(out, unixFindEngine{})
		case "grep":
			out = append(out, grepEngine{})
		case "combined":
			out = append(out, combinedEngine{})
		case "fd":
			out = append(out, fdEngine{})
		case "rg":
			out = append(out, rgEngine{})
		case "hybrid":
			out = append(out, hybridEngine{})
		}
	}
	return out
}
