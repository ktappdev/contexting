package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// BenchOutput is the result returned by runBench.
type BenchOutput struct {
	Cases      []EvalCase       `json:"cases"`
	Results    [][]EngineResult `json:"results"` // results[i] is per-engine for cases[i]
	Summary    []EngineSummary  `json:"summary"`
	Misses     []BenchMiss      `json:"misses,omitempty"`
	IndexLoadMs int64           `json:"index_load_ms"`
}

// BenchMiss records which query failed to match an expected path in an engine.
type BenchMiss struct {
	Query     string `json:"query"`
	Engine    string `json:"engine"`
	Expected  string `json:"expected"`
}

// CategoryReport aggregates results for one category across all engines.
type CategoryReport struct {
	Category  string          `json:"category"`
	CaseCount int             `json:"case_count"`
	Engines   []EngineSummary `json:"engines"`
}

// FullReport contains per-category reports and overall summary.
type FullReport struct {
	Categories  []CategoryReport `json:"categories"`
	Overall     []EngineSummary  `json:"overall"`
	IndexLoadMs int64            `json:"index_load_ms"`
}

// fullReportWithLoad returns a FullReport with index load time included.
func fullReportWithLoad(cases []EvalCase, results [][]EngineResult, indexLoadMs int64) FullReport {
	return FullReport{
		Categories:  computeCategorySummaries(cases, results),
		Overall:     computeEngineSummaries(cases, results),
		IndexLoadMs: indexLoadMs,
	}
}

// printBenchSummary prints per-query tables and a final aggregate summary.
func printBenchSummary(out BenchOutput) {
	if len(out.Cases) == 0 {
		fmt.Println("No cases to benchmark.")
		return
	}

	for i, c := range out.Cases {
		printBenchTable(c.Query, out.Results[i])
	}

	totalCases := len(out.Cases)
	fmt.Printf("\n=== Summary (%d cases) ===\n", totalCases)
	fmt.Printf("%-15s %-7s %-13s %-10s %-8s %-8s %-7s %-7s %-7s %-10s %-7s\n", "Engine", "Recall", "Avg Results", "Avg Time", "p50", "p95", "Hit@1", "Hit@3", "Hit@5", "Avg Tokens", "Noise")
	for _, s := range out.Summary {
		fmt.Printf("%-15s %-7s %-13.1f %-10s %-8s %-8s %-7s %-7s %-7s %-10.1f %-7.2f\n",
			s.EngineName,
			fmt.Sprintf("%.0f%%", s.Recall*100),
			s.AvgResults,
			fmt.Sprintf("%.0fms", s.AvgTimeMs),
			fmt.Sprintf("%dms", s.P50TimeMs),
			fmt.Sprintf("%dms", s.P95TimeMs),
			fmt.Sprintf("%.0f%%", s.HitAt1*100),
			fmt.Sprintf("%.0f%%", s.HitAt3*100),
			fmt.Sprintf("%.0f%%", s.HitAt5*100),
			s.AvgTokens,
			s.AvgNoiseRatio,
		)
	}

	if len(out.Misses) > 0 {
		fmt.Println("\n=== Misses ===")
		for _, m := range out.Misses {
			fmt.Printf("Query: %q  Engine: %s  Expected: %s\n", m.Query, m.Engine, m.Expected)
		}
	}
	for _, s := range out.Summary {
		if s.EngineName == "grep" {
			fmt.Println("\nNote: grep returns alphabetically sorted results, so Hit@1 is near 0% by design. Use recall to evaluate grep content matching.")
			break
		}
	}
}

func printBenchTable(query string, results []EngineResult) {
	fmt.Printf("\nQuery: %s\n", query)
	fmt.Printf("%-12s %-7s %-5s %-8s %-8s %-7s %-7s %-7s %-10s\n", "Engine", "Found", "Rank", "Results", "Time", "Hit@1", "Hit@3", "Hit@5", "Tokens")
	for _, r := range results {
		rankStr := "-"
		if r.Rank > 0 {
			rankStr = fmt.Sprintf("%d", r.Rank)
		}
		foundStr := "no"
		if r.Found {
			foundStr = "yes"
		}
		hit1Str := "no"
		if r.HitAt1 {
			hit1Str = "yes"
		}
		hit3Str := "no"
		if r.HitAt3 {
			hit3Str = "yes"
		}
		hit5Str := "no"
		if r.HitAt5 {
			hit5Str = "yes"
		}
		fmt.Printf("%-12s %-7s %-5s %-8d %-8s %-7s %-7s %-7s %-10d\n", r.EngineName, foundStr, rankStr, r.TotalHits, fmt.Sprintf("%dms", r.TimeMs), hit1Str, hit3Str, hit5Str, r.Tokens)
	}
}

// resultsToJSONBench marshals the bench output to a pretty-printed JSON string.
func resultsToJSONBench(out BenchOutput) (string, error) {
	bytes, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal bench results: %w", err)
	}
	return string(bytes), nil
}

// computeEngineSummaries builds aggregate statistics from per-engine results.
func computeEngineSummaries(cases []EvalCase, results [][]EngineResult) []EngineSummary {
	if len(results) == 0 || len(results[0]) == 0 {
		return nil
	}

	engineCount := len(results[0])
	summaries := make([]EngineSummary, engineCount)
	for i := 0; i < engineCount; i++ {
		summaries[i].EngineName = results[0][i].EngineName
	}

	for caseIdx, c := range cases {
		validCase := strings.TrimSpace(c.Query) != "" && len(c.ExpectAny) > 0
		for engIdx := range results[caseIdx] {
			s := &summaries[engIdx]
			res := results[caseIdx][engIdx]
			if validCase {
				s.Cases++
				if caseMatched(c, res) {
					s.Recall += 1.0
				} else {
					s.FailedCases++
				}
				if res.HitAt1 {
					s.HitAt1 += 1.0
				}
				if res.HitAt3 {
					s.HitAt3 += 1.0
				}
				if res.HitAt5 {
					s.HitAt5 += 1.0
				}
				s.AvgTokens += float64(res.Tokens)
				s.AvgNoiseRatio += res.NoiseRatio
			} else {
				s.FailedCases++
			}
			s.AvgResults += float64(res.TotalHits)
			s.AvgTimeMs += float64(res.TimeMs)
		}
	}

	for i := range summaries {
		s := &summaries[i]
		total := len(cases)
		if total > 0 {
			s.AvgResults /= float64(total)
			s.AvgTimeMs /= float64(total)
		}
		if s.Cases > 0 {
			s.Recall /= float64(s.Cases)
			s.HitAt1 /= float64(s.Cases)
			s.HitAt3 /= float64(s.Cases)
			s.HitAt5 /= float64(s.Cases)
			s.AvgTokens /= float64(s.Cases)
			s.AvgNoiseRatio /= float64(s.Cases)
		}
		s.P50TimeMs, s.P95TimeMs = percentileTimes(results, i)
	}

	return summaries
}

func caseMatched(c EvalCase, res EngineResult) bool {
	if res.EngineName == "ctxt" {
		return res.Rank > 0
	}
	for _, expected := range c.ExpectAny {
		for _, p := range res.Paths {
			if pathMatchesExpected(p, expected) {
				return true
			}
		}
	}
	return false
}

func percentileTimes(results [][]EngineResult, engineIdx int) (int64, int64) {
	times := make([]int64, 0, len(results))
	for i := range results {
		if len(results[i]) <= engineIdx {
			continue
		}
		times = append(times, results[i][engineIdx].TimeMs)
	}
	if len(times) == 0 {
		return 0, 0
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	return percentile(times, 0.50), percentile(times, 0.95)
}

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	pos := p * float64(len(sorted)-1)
	lower := int(pos)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[lower]
	}
	frac := pos - float64(lower)
	return int64(float64(sorted[lower]) + frac*float64(sorted[upper]-sorted[lower]))
}

// computeBenchMisses returns misses for each engine where the expected path was not found.
func computeBenchMisses(cases []EvalCase, results [][]EngineResult) []BenchMiss {
	misses := make([]BenchMiss, 0)
	for i, c := range cases {
		if strings.TrimSpace(c.Query) == "" || len(c.ExpectAny) == 0 {
			continue
		}
		for _, res := range results[i] {
			if !caseMatched(c, res) {
				misses = append(misses, BenchMiss{
					Query:    c.Query,
					Engine:   res.EngineName,
					Expected: strings.Join(c.ExpectAny, ", "),
				})
			}
		}
	}
	return misses
}

func pathMatchesExpected(resultPath, expected string) bool {
	normalizedResult := normalizeEvalPath(resultPath)
	normalizedExpected := normalizeEvalPath(expected)
	return normalizedResult == normalizedExpected || strings.HasSuffix(normalizedResult, normalizedExpected)
}

// groupResultsByCategory groups EngineResults by their corresponding case's Category.
// Returns map of category -> [][]EngineResult (maintaining per-case structure).
func groupResultsByCategory(results [][]EngineResult, cases []EvalCase) map[string][][]EngineResult {
	grouped := make(map[string][][]EngineResult)
	for caseIdx, c := range cases {
		category := c.Category
		if category == "" {
			category = "(uncategorized)"
		}
		grouped[category] = append(grouped[category], results[caseIdx])
	}
	return grouped
}

// groupCasesByCategory groups cases by their Category.
func groupCasesByCategory(cases []EvalCase) map[string][]EvalCase {
	grouped := make(map[string][]EvalCase)
	for _, c := range cases {
		category := c.Category
		if category == "" {
			category = "(uncategorized)"
		}
		grouped[category] = append(grouped[category], c)
	}
	return grouped
}

// computeCategorySummaries builds CategoryReport entries for each category.
func computeCategorySummaries(cases []EvalCase, results [][]EngineResult) []CategoryReport {
	// Group cases by category
	casesByCategory := groupCasesByCategory(cases)

	// For each category, filter cases and results
	reports := make([]CategoryReport, 0, len(casesByCategory))
	for category, categoryCases := range casesByCategory {
		// Find indices of cases in this category
		categoryIndices := make([]int, 0, len(categoryCases))
		for caseIdx, c := range cases {
			cat := c.Category
			if cat == "" {
				cat = "(uncategorized)"
			}
			if cat == category {
				categoryIndices = append(categoryIndices, caseIdx)
			}
		}

		// Filter results to just this category
		filteredResults := make([][]EngineResult, 0, len(categoryIndices))
		for _, idx := range categoryIndices {
			filteredResults = append(filteredResults, results[idx])
		}

		// Compute summaries for this category
		summaries := computeEngineSummaries(categoryCases, filteredResults)

		reports = append(reports, CategoryReport{
			Category:  category,
			CaseCount: len(categoryCases),
			Engines:   summaries,
		})
	}

	// Sort categories alphabetically
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Category < reports[j].Category
	})

	return reports
}

// printCategoryReport prints a category-grouped head-to-head report.
func printCategoryReport(cases []EvalCase, results [][]EngineResult) {
	if len(cases) == 0 {
		fmt.Println("No cases to benchmark.")
		return
	}

	// Print per-query tables first
	for i, c := range cases {
		printBenchTable(c.Query, results[i])
	}

	// Compute category reports
	categoryReports := computeCategorySummaries(cases, results)

	// Print each category
	for _, report := range categoryReports {
		fmt.Printf("\nCategory: %s (%d cases)\n\n", report.Category, report.CaseCount)
		fmt.Printf("%-15s %-7s %-7s %-7s %-7s %-10s %-10s %-7s\n", "Tool", "Hit@1", "Hit@3", "Hit@5", "Recall", "Avg Time", "Avg Tokens", "Noise")
		fmt.Printf("%-15s %-7s %-7s %-7s %-7s %-10s %-10s %-7s\n", "──────────────", "───────", "───────", "───────", "───────", "──────────", "──────────", "───────")
		for _, s := range report.Engines {
			fmt.Printf("%-15s %-7.2f %-7.2f %-7.2f %-7.2f %-10s %-10d %-7.2f\n",
				s.EngineName,
				s.HitAt1,
				s.HitAt3,
				s.HitAt5,
				s.Recall*100,
				fmt.Sprintf("%.0fms", s.AvgTimeMs),
				int(s.AvgTokens),
				s.AvgNoiseRatio,
			)
		}
	}

	// Print overall summary
	overallSummaries := computeEngineSummaries(cases, results)
	fmt.Printf("\n═══ Overall (%d cases) ═══\n\n", len(cases))
	fmt.Printf("%-15s %-7s %-7s %-7s %-7s %-10s %-10s %-7s\n", "Tool", "Hit@1", "Hit@3", "Hit@5", "Recall", "Avg Time", "Avg Tokens", "Noise")
	fmt.Printf("%-15s %-7s %-7s %-7s %-7s %-10s %-10s %-7s\n", "──────────────", "───────", "───────", "───────", "───────", "──────────", "──────────", "───────")
	for _, s := range overallSummaries {
		fmt.Printf("%-15s %-7.2f %-7.2f %-7.2f %-7.2f %-10s %-10d %-7.2f\n",
			s.EngineName,
			s.HitAt1,
			s.HitAt3,
			s.HitAt5,
			s.Recall*100,
			fmt.Sprintf("%.0fms", s.AvgTimeMs),
			int(s.AvgTokens),
			s.AvgNoiseRatio,
		)
	}
	for _, s := range overallSummaries {
		if s.EngineName == "grep" {
			fmt.Println("\nNote: grep returns alphabetically sorted results, so Hit@1 is near 0% by design. Use recall to evaluate grep content matching.")
			break
		}
	}
}

// categoryReportToJSON builds a FullReport and marshals to JSON.
func categoryReportToJSON(cases []EvalCase, results [][]EngineResult, indexLoadMs int64) (string, error) {
	fullReport := fullReportWithLoad(cases, results, indexLoadMs)

	bytes, err := json.MarshalIndent(fullReport, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal category report: %w", err)
	}
	return string(bytes), nil
}
