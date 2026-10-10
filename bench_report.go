package contexting

import (
	"encoding/json"
	"fmt"
)

type BenchOutput struct {
	CorpusPreparationMs float64          `json:"corpus_preparation_ms"`
	ReportVersion       int              `json:"report_version"`
	Methodology         BenchMethodology `json:"methodology"`
	Cases               []EvalCase       `json:"cases"`
	Results             [][]EngineResult `json:"results"`
	Summary             []EngineSummary  `json:"summary"`
	Categories          []CategoryReport `json:"categories,omitempty"`
	Misses              []BenchMiss      `json:"misses,omitempty"`
	IndexLoadMs         float64          `json:"index_load_ms"`
}

type BenchMiss struct {
	Query    string `json:"query"`
	Engine   string `json:"engine"`
	Expected string `json:"expected"`
}
type CategoryReport struct {
	Category  string          `json:"category"`
	CaseCount int             `json:"case_count"`
	Engines   []EngineSummary `json:"engines"`
}

func printBenchSummary(out BenchOutput) {
	if len(out.Cases) == 0 {
		fmt.Println("No cases to benchmark.")
		return
	}
	for i, entry := range out.Cases {
		printBenchTable(entry.Query, out.Results[i])
	}
	fmt.Printf("\nSummary (%d cases)\n", len(out.Cases))
	printEngineSummaries(out.Summary)
	if len(out.Misses) > 0 {
		fmt.Println("\nMisses:")
		for _, miss := range out.Misses {
			fmt.Printf("  %q [%s] expected %s\n", miss.Query, miss.Engine, miss.Expected)
		}
	}
	printBenchNotes()
}

func printBenchTable(query string, results []EngineResult) {
	fmt.Printf("\nQuery: %s\n", query)
	fmt.Printf("%-10s %-13s %-6s %-7s %-10s %-7s %-7s %-7s %-10s\n", "Engine", "Order", "Rank", "Results", "Median ms", "Hit@1", "Hit@3", "Hit@5", "Path toks~")
	for _, result := range results {
		if result.Error != "" {
			fmt.Printf("%-10s ERROR: %s\n", result.EngineName, result.Error)
			continue
		}
		fmt.Printf("%-10s %-13s %-6d %-7d %-10.3f %-7t %-7t %-7t %-10d\n", result.EngineName, result.Ordering, result.Rank, result.TotalHits, result.TimeMs, result.HitAt1, result.HitAt3, result.HitAt5, result.EstimatedPathTokens)
	}
}

func printEngineSummaries(summaries []EngineSummary) {
	fmt.Printf("%-10s %-13s %-9s %-8s %-8s %-8s %-9s %-9s %-9s %-10s %-8s %-6s\n", "Engine", "Order", "Success%", "Hit@1%", "Hit@3%", "Hit@5%", "Avg ms", "p50 ms", "p95 ms", "Path toks~", "Noise%", "Errors")
	for _, summary := range summaries {
		fmt.Printf("%-10s %-13s %-9.2f %-8.2f %-8.2f %-8.2f %-8.3f %-9.3f %-9.3f %-10.2f %-8.2f %-6d\n", summary.EngineName, summary.Ordering, 100*summary.SuccessRate, 100*summary.HitAt1, 100*summary.HitAt3, 100*summary.HitAt5, summary.AvgTimeMs, summary.P50TimeMs, summary.P95TimeMs, summary.AvgEstimatedPathTokens, 100*summary.AvgNoiseRatio, summary.ExecutionErrors)
	}
}

func printBenchNotes() {
	fmt.Println("\nSuccess = queries with any expected file within the shared result limit.")
	fmt.Println("Alphabetical Hit@K measures output position, not relevance ranking.")
	fmt.Println("Path toks~ = path bytes / 4 only; excludes JSON, metadata, and file contents.")
	fmt.Println("Timings are repeated adapter calls: ctxt runs in process; external engines include subprocess startup. Index load is separate.")
}

func printCategoryReport(cases []EvalCase, results [][]EngineResult) {
	if len(cases) == 0 {
		fmt.Println("No cases to benchmark.")
		return
	}
	for i, entry := range cases {
		printBenchTable(entry.Query, results[i])
	}
	for _, category := range computeCategorySummaries(cases, results) {
		fmt.Printf("\nCategory: %s (%d cases)\n", category.Category, category.CaseCount)
		printEngineSummaries(category.Engines)
	}
	fmt.Println("\nOverall:")
	printEngineSummaries(computeEngineSummaries(cases, results))
	printBenchNotes()
}

func resultsToJSONBench(out BenchOutput) (string, error) {
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal bench results: %w", err)
	}
	return string(data), nil
}
