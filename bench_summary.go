package contexting

import (
	"sort"
	"strings"
)

func computeEngineSummaries(cases []EvalCase, results [][]EngineResult) []EngineSummary {
	if len(results) == 0 {
		return nil
	}
	summaries := make([]EngineSummary, len(results[0]))
	for i, first := range results[0] {
		summaries[i].EngineName = first.EngineName
		summaries[i].Ordering = first.Ordering
		summaries[i].Execution = first.Execution
	}
	for caseIdx, entry := range cases {
		for i, result := range results[caseIdx] {
			summary := &summaries[i]
			if !validBenchCase(entry) {
				summary.FailedCases++
				continue
			}
			summary.Cases++
			if result.Error != "" {
				summary.ExecutionErrors++
				continue
			}
			if caseMatched(entry, result) {
				summary.SuccessRate++
			} else {
				summary.FailedCases++
			}
			if result.HitAt1 {
				summary.HitAt1++
			}
			if result.HitAt3 {
				summary.HitAt3++
			}
			if result.HitAt5 {
				summary.HitAt5++
			}
			summary.AvgResults += float64(result.TotalHits)
			summary.AvgEstimatedPathTokens += float64(result.EstimatedPathTokens)
			summary.AvgNoiseRatio += result.NoiseRatio
		}
	}
	for i := range summaries {
		summary := &summaries[i]
		// Execution failures count against success rates, but are excluded from
		// output-size and latency averages. Never report missing tools as fast misses.
		if summary.Cases > 0 {
			count := float64(summary.Cases)
			summary.SuccessRate /= count
			summary.HitAt1 /= count
			summary.HitAt3 /= count
			summary.HitAt5 /= count
		}
		completed := summary.Cases - summary.ExecutionErrors
		if completed > 0 {
			count := float64(completed)
			summary.AvgResults /= count
			summary.AvgEstimatedPathTokens /= count
			summary.AvgNoiseRatio /= count
		}
		samples := timingSamples(results, i)
		summary.TimingSamples = len(samples)
		for _, sample := range samples {
			summary.AvgTimeMs += sample
		}
		if len(samples) > 0 {
			summary.AvgTimeMs /= float64(len(samples))
		}
		summary.P50TimeMs, summary.P95TimeMs = percentile(samples, 0.5), percentile(samples, 0.95)
	}
	return summaries
}

func caseMatched(entry EvalCase, result EngineResult) bool {
	if result.Error != "" {
		return false
	}
	for _, candidate := range result.Paths {
		for _, want := range entry.ExpectAny {
			if pathMatchesExpected(candidate, want) {
				return true
			}
		}
	}
	return false
}

func timingSamples(results [][]EngineResult, engine int) []float64 {
	var samples []float64
	for _, row := range results {
		if engine >= len(row) || row[engine].Error != "" {
			continue
		}
		samples = append(samples, row[engine].SamplesMs...)
	}
	sort.Float64s(samples)
	return samples
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	position := p * float64(len(sorted)-1)
	lower := int(position)
	upper := lower + 1
	if upper >= len(sorted) {
		return sorted[lower]
	}
	return sorted[lower] + (position-float64(lower))*(sorted[upper]-sorted[lower])
}

func computeBenchMisses(cases []EvalCase, results [][]EngineResult) []BenchMiss {
	var misses []BenchMiss
	for i, entry := range cases {
		if !validBenchCase(entry) {
			continue
		}
		for _, result := range results[i] {
			if result.Error == "" && !caseMatched(entry, result) {
				misses = append(misses, BenchMiss{Query: entry.Query, Engine: result.EngineName, Expected: strings.Join(entry.ExpectAny, ", ")})
			}
		}
	}
	return misses
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
