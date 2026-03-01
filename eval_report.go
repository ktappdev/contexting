package main

import (
	"encoding/json"
	"fmt"
)

func printEvalSummary(summary EvalSummary) {
	fmt.Printf("Cases: %d (scored=%d, invalid=%d)\n", summary.Cases, summary.ScoredCases, summary.FailedCases)
	fmt.Printf("Hit@1: %.2f%% (%d)\n", summary.HitAt1*100, summary.Top1Hits)
	fmt.Printf("Hit@3: %.2f%% (%d)\n", summary.HitAt3*100, summary.Top3Hits)
	fmt.Printf("Hit@5: %.2f%% (%d)\n", summary.HitAt5*100, summary.Top5Hits)
	fmt.Printf("MRR: %.4f\n", summary.MRR)
}

func printFailedCases(results []EvalResult) {
	printed := 0
	for _, result := range results {
		if result.MatchedAt > 0 {
			continue
		}
		if printed == 0 {
			fmt.Println("Misses:")
		}
		printed++
		fmt.Printf("- query=%q expected=%v\n", result.Case.Query, result.Case.ExpectAny)
		for i, candidate := range result.TopPaths {
			fmt.Printf("  %d) %s score=%d\n", i+1, candidate.Path, candidate.Score)
		}
	}
	if printed == 0 {
		fmt.Println("Misses: none")
	}
}

func resultsToJSONEval(value any) (string, error) {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
