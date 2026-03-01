package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type EvalCase struct {
	Query     string   `json:"query"`
	ExpectAny []string `json:"expect_any"`
}

type EvalSummary struct {
	Cases       int     `json:"cases"`
	Top1Hits    int     `json:"top1_hits"`
	Top3Hits    int     `json:"top3_hits"`
	Top5Hits    int     `json:"top5_hits"`
	HitAt1      float64 `json:"hit_at_1"`
	HitAt3      float64 `json:"hit_at_3"`
	HitAt5      float64 `json:"hit_at_5"`
	MRR         float64 `json:"mrr"`
	FailedCases int     `json:"failed_cases"`
	ScoredCases int     `json:"scored_cases"`
}

type EvalResult struct {
	Case      EvalCase       `json:"case"`
	MatchedAt int            `json:"matched_at"`
	TopPaths  []SearchResult `json:"top_paths"`
}

func LoadEvalCases(path string) ([]EvalCase, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read eval cases: %w", err)
	}

	var cases []EvalCase
	if err := json.Unmarshal(bytes, &cases); err != nil {
		return nil, fmt.Errorf("parse eval cases: %w", err)
	}
	return cases, nil
}

func EvaluateSearch(index *ContextIndex, cases []EvalCase, opts SearchOptions) (EvalSummary, []EvalResult) {
	summary := EvalSummary{Cases: len(cases)}
	results := make([]EvalResult, 0, len(cases))
	if len(cases) == 0 || index == nil || index.Tree == nil {
		return summary, results
	}

	var reciprocalRankTotal float64
	for _, evalCase := range cases {
		if strings.TrimSpace(evalCase.Query) == "" || len(evalCase.ExpectAny) == 0 {
			summary.FailedCases++
			results = append(results, EvalResult{Case: evalCase, MatchedAt: -1})
			continue
		}

		summary.ScoredCases++
		searchResults := SearchHintsWithOptions(index, evalCase.Query, opts)
		matchedAt := firstMatchRank(searchResults, evalCase.ExpectAny)

		if matchedAt == 1 {
			summary.Top1Hits++
		}
		if matchedAt > 0 && matchedAt <= 3 {
			summary.Top3Hits++
		}
		if matchedAt > 0 && matchedAt <= 5 {
			summary.Top5Hits++
		}
		if matchedAt > 0 {
			reciprocalRankTotal += 1.0 / float64(matchedAt)
		}

		results = append(results, EvalResult{
			Case:      evalCase,
			MatchedAt: matchedAt,
			TopPaths:  searchResults,
		})
	}

	scored := float64(summary.ScoredCases)
	if scored > 0 {
		summary.HitAt1 = float64(summary.Top1Hits) / scored
		summary.HitAt3 = float64(summary.Top3Hits) / scored
		summary.HitAt5 = float64(summary.Top5Hits) / scored
		summary.MRR = reciprocalRankTotal / scored
	}

	return summary, results
}

func firstMatchRank(results []SearchResult, expectAny []string) int {
	normalizedExpected := make([]string, 0, len(expectAny))
	for _, expected := range expectAny {
		normalizedExpected = append(normalizedExpected, normalizeEvalPath(expected))
	}

	for i, result := range results {
		normalizedResult := normalizeEvalPath(result.Path)
		for _, expected := range normalizedExpected {
			if expected == normalizedResult || strings.HasSuffix(normalizedResult, expected) {
				return i + 1
			}
		}
	}
	return -1
}

func normalizeEvalPath(path string) string {
	return filepath.ToSlash(strings.TrimSpace(strings.ToLower(path)))
}
