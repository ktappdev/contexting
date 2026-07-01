package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SearchEngine is the contract every bench engine implements.
type SearchEngine interface {
	Name() string
	Search(query string, expectAny []string, index *ContextIndex, opts SearchOptions, grepMaxBytes int) EngineResult
}

// EngineResult captures timing + outcome for one engine on one query.
type EngineResult struct {
	EngineName string   `json:"engine_name"`
	Found      bool     `json:"found"`
	Rank       int      `json:"rank"`       // 1-based; -1 if not found (only contexting ranks)
	TotalHits  int      `json:"total_hits"` // number of paths returned
	Paths      []string `json:"paths,omitempty"`
	TimeMs     int64    `json:"time_ms"`
	Error      string   `json:"error,omitempty"` // empty if no error
	Chars      int      `json:"chars"`           // total chars across all returned paths
	Tokens     int      `json:"tokens"`          // chars/4 heuristic
	HitAt1     bool     `json:"hit_at_1"`
	HitAt3     bool     `json:"hit_at_3"`
	HitAt5     bool     `json:"hit_at_5"`
	NoiseRatio float64  `json:"noise_ratio"`     // 1.0 - (relevantResults / totalResults)
}

// EngineSummary aggregates results for one engine across all cases.
type EngineSummary struct {
	EngineName    string  `json:"engine_name"`
	Cases         int     `json:"cases"`
	Recall        float64 `json:"recall"`
	AvgResults    float64 `json:"avg_results"`
	AvgTimeMs     float64 `json:"avg_time_ms"`
	P50TimeMs     int64   `json:"p50_time_ms"`
	P95TimeMs     int64   `json:"p95_time_ms"`
	FailedCases   int     `json:"failed_cases"`
	HitAt1        float64 `json:"hit_at_1"`
	HitAt3        float64 `json:"hit_at_3"`
	HitAt5        float64 `json:"hit_at_5"`
	AvgTokens     float64 `json:"avg_tokens"`
	AvgNoiseRatio float64 `json:"avg_noise_ratio"`
}

type contextingEngine struct{}

func (contextingEngine) Name() string { return "contexting" }

func (contextingEngine) Search(query string, expectAny []string, index *ContextIndex, opts SearchOptions, _ int) EngineResult {
	start := time.Now()
	results := SearchHintsWithOptions(index, query, opts)
	rank := firstMatchRank(results, expectAny)
	found := rank > 0
	if !found {
		rank = -1
	}
	paths := make([]string, 0, len(results))
	for _, r := range results {
		paths = append(paths, r.Path)
	}
	hit1, hit3, hit5, _ := computeHitAtK(paths, expectAny)
	chars, tokens := countTokens(paths)
	noiseRatio := computeNoiseRatio(paths, expectAny)
	return EngineResult{
		EngineName: "contexting",
		Found:      found,
		Rank:       rank,
		TotalHits:  len(results),
		Paths:      paths,
		TimeMs:     time.Since(start).Milliseconds(),
		Chars:      chars,
		Tokens:     tokens,
		HitAt1:     hit1,
		HitAt3:     hit3,
		HitAt5:     hit5,
		NoiseRatio: noiseRatio,
	}
}

type findEngine struct{}

func (findEngine) Name() string { return "find" }

func (findEngine) Search(query string, expectAny []string, index *ContextIndex, _ SearchOptions, _ int) EngineResult {
	start := time.Now()
	ignored, err := BuildIgnoreMapForRoot(index.RootPath, nil)
	if err != nil {
		return EngineResult{
			EngineName: "find",
			Error:      fmt.Sprintf("build ignore map: %v", err),
		}
	}
	queryTokens := tokenize(query)
	results := make([]string, 0)
	err = filepath.WalkDir(index.RootPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, walkErr := filepath.Rel(index.RootPath, path)
		if walkErr != nil {
			return walkErr
		}
		if rel == "." {
			return nil
		}
		if shouldIgnorePath(rel, d.Name(), ignored) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if tokensMatchPath(rel, d.Name(), queryTokens) {
			results = append(results, rel)
		}
		return nil
	})
	if err != nil {
		return EngineResult{
			EngineName: "find",
			Error:      fmt.Sprintf("walk dir: %v", err),
		}
	}
	sort.Strings(results)
	hit1, hit3, hit5, firstRank := computeHitAtK(results, expectAny)
	chars, tokens := countTokens(results)
	noiseRatio := computeNoiseRatio(results, expectAny)
	return EngineResult{
		EngineName: "find",
		Found:      firstRank > 0,
		Rank:       -1,
		TotalHits:  len(results),
		Paths:      results,
		TimeMs:     time.Since(start).Milliseconds(),
		Chars:      chars,
		Tokens:     tokens,
		HitAt1:     hit1,
		HitAt3:     hit3,
		HitAt5:     hit5,
		NoiseRatio: noiseRatio,
	}
}

type grepEngine struct{}

func (grepEngine) Name() string { return "grep" }

func (grepEngine) Search(query string, expectAny []string, index *ContextIndex, _ SearchOptions, grepMaxBytes int) EngineResult {
	start := time.Now()
	ignored, err := BuildIgnoreMapForRoot(index.RootPath, nil)
	if err != nil {
		return EngineResult{
			EngineName: "grep",
			Error:      fmt.Sprintf("build ignore map: %v", err),
		}
	}
	if grepMaxBytes <= 0 {
		grepMaxBytes = 1048576
	}
	queryTokens := tokenize(query)
	results := make([]string, 0)
	err = filepath.WalkDir(index.RootPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, walkErr := filepath.Rel(index.RootPath, path)
		if walkErr != nil {
			return walkErr
		}
		if rel == "." {
			return nil
		}
		if shouldIgnorePath(rel, d.Name(), ignored) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			return nil
		}
		if info.Size() > int64(grepMaxBytes) {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if hasNullByte(content, 512) {
			return nil
		}
		if tokensMatchContent(string(content), queryTokens) {
			results = append(results, rel)
		}
		return nil
	})
	if err != nil {
		return EngineResult{
			EngineName: "grep",
			Error:      fmt.Sprintf("walk dir: %v", err),
		}
	}
	sort.Strings(results)
	hit1, hit3, hit5, firstRank := computeHitAtK(results, expectAny)
	chars, tokens := countTokens(results)
	noiseRatio := computeNoiseRatio(results, expectAny)
	return EngineResult{
		EngineName: "grep",
		Found:      firstRank > 0,
		Rank:       -1,
		TotalHits:  len(results),
		Paths:      results,
		TimeMs:     time.Since(start).Milliseconds(),
		Chars:      chars,
		Tokens:     tokens,
		HitAt1:     hit1,
		HitAt3:     hit3,
		HitAt5:     hit5,
		NoiseRatio: noiseRatio,
	}
}

type combinedEngine struct {
	find findEngine
	grep grepEngine
}

func (combinedEngine) Name() string { return "combined" }

func (c combinedEngine) Search(query string, expectAny []string, index *ContextIndex, opts SearchOptions, grepMaxBytes int) EngineResult {
	start := time.Now()
	findRes := c.find.Search(query, expectAny, index, opts, grepMaxBytes)
	grepRes := c.grep.Search(query, expectAny, index, opts, grepMaxBytes)
	union := make(map[string]struct{}, findRes.TotalHits+grepRes.TotalHits)
	if findRes.Error == "" {
		for _, p := range findRes.Paths {
			union[p] = struct{}{}
		}
	}
	if grepRes.Error == "" {
		for _, p := range grepRes.Paths {
			union[p] = struct{}{}
		}
	}
	combined := make([]string, 0, len(union))
	for p := range union {
		combined = append(combined, p)
	}
	sort.Strings(combined)
	hit1, hit3, hit5, firstRank := computeHitAtK(combined, expectAny)
	chars, tokens := countTokens(combined)
	noiseRatio := computeNoiseRatio(combined, expectAny)
	return EngineResult{
		EngineName: "combined",
		Found:      firstRank > 0,
		Rank:       -1,
		TotalHits:  len(combined),
		Paths:      combined,
		TimeMs:     time.Since(start).Milliseconds(),
		Chars:      chars,
		Tokens:     tokens,
		HitAt1:     hit1,
		HitAt3:     hit3,
		HitAt5:     hit5,
		NoiseRatio: noiseRatio,
	}
}

func tokensMatchPath(relPath, baseName string, tokens []string) bool {
	relLower := strings.ToLower(filepath.ToSlash(relPath))
	baseLower := strings.ToLower(baseName)
	for _, token := range tokens {
		if strings.Contains(baseLower, token) || strings.Contains(relLower, token) {
			return true
		}
	}
	return false
}

func tokensMatchContent(content string, tokens []string) bool {
	lower := strings.ToLower(content)
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}



func hasNullByte(data []byte, limit int) bool {
	if limit > len(data) {
		limit = len(data)
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// computeHitAtK returns Hit@1, Hit@3, Hit@5 booleans and the 1-based rank of first expected match.
// For contexting: paths are already ranked (search results order).
// For find/grep/combined: paths are sorted alphabetically — rank is position in that sorted list.
func computeHitAtK(paths []string, expectAny []string) (hit1, hit3, hit5 bool, firstRank int) {
	normalizedExpected := make([]string, 0, len(expectAny))
	for _, expected := range expectAny {
		normalizedExpected = append(normalizedExpected, normalizeEvalPath(expected))
	}

	for i, path := range paths {
		normalizedPath := normalizeEvalPath(path)
		for _, expected := range normalizedExpected {
			if expected == normalizedPath || strings.HasSuffix(normalizedPath, expected) {
				rank := i + 1
				hit1 = rank == 1
				hit3 = rank > 0 && rank <= 3
				hit5 = rank > 0 && rank <= 5
				return hit1, hit3, hit5, rank
			}
		}
	}
	return false, false, false, -1
}

// countTokens returns total character count and estimated token count (chars/4) for a list of paths.
func countTokens(paths []string) (chars int, tokens int) {
	for _, path := range paths {
		chars += len(path)
	}
	tokens = chars / 4
	return chars, tokens
}

// computeNoiseRatio returns 1.0 - (relevantPaths / totalPaths). 0 if no paths.
func computeNoiseRatio(paths []string, expectAny []string) float64 {
	total := len(paths)
	if total == 0 {
		return 0
	}
	relevant := countRelevantPaths(paths, expectAny)
	return 1.0 - float64(relevant)/float64(total)
}

// countRelevantPaths counts how many paths match any expected path.
func countRelevantPaths(paths []string, expectAny []string) int {
	normalizedExpected := make([]string, 0, len(expectAny))
	for _, expected := range expectAny {
		normalizedExpected = append(normalizedExpected, normalizeEvalPath(expected))
	}

	relevant := 0
	for _, path := range paths {
		normalizedPath := normalizeEvalPath(path)
		for _, expected := range normalizedExpected {
			if expected == normalizedPath || strings.HasSuffix(normalizedPath, expected) {
				relevant++
				break
			}
		}
	}
	return relevant
}

// knownEngines returns the list of supported engine names in default order.
func knownEngines() []string {
	return []string{"contexting", "find", "grep", "combined"}
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
		case "contexting":
			out = append(out, contextingEngine{})
		case "find":
			out = append(out, findEngine{})
		case "grep":
			out = append(out, grepEngine{})
		case "combined":
			out = append(out, combinedEngine{})
		}
	}
	return out
}
