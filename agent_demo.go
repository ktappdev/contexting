package main

import "fmt"

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
