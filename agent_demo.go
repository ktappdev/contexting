package main

import "fmt"

// LoadAndSearchHints is a lightweight agent-facing helper:
// load context JSON and return ranked path hints for a natural-language query.
func LoadAndSearchHints(indexPath string, query string, limit int) ([]SearchResult, error) {
	index, err := LoadContextIndex(indexPath)
	if err != nil {
		return nil, err
	}
	return SearchHintsWithOptions(index, query, SearchOptions{Limit: limit}), nil
}

func printSearchResults(results []SearchResult) {
	if len(results) == 0 {
		fmt.Println("No matches found")
		return
	}

	for i, result := range results {
		fmt.Printf("%d. %s (%s) score=%d matches=%v\n", i+1, result.Path, result.Type, result.Score, result.Matches)
		if len(result.Breakdown) > 0 {
			fmt.Printf("   breakdown=%v\n", result.Breakdown)
		}
	}
}
