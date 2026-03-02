package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type DirectorySummary struct {
	Path       string         `json:"path"`
	TotalScore int            `json:"total_score"`
	MatchCount int            `json:"match_count"`
	Rationale  string         `json:"rationale"`
	TopMatches []SearchResult `json:"top_matches"`
}

func SummarizeDirectories(results []SearchResult, dirLimit int, drillLimit int) []DirectorySummary {
	if dirLimit <= 0 {
		dirLimit = 5
	}
	if drillLimit <= 0 {
		drillLimit = 3
	}
	if len(results) == 0 {
		return nil
	}

	type aggregate struct {
		Path       string
		TotalScore int
		MatchCount int
		TopMatches []SearchResult
		Tokens     map[string]struct{}
	}

	byDir := make(map[string]*aggregate)
	for _, result := range results {
		dir := directoryForResultPath(result)
		entry := byDir[dir]
		if entry == nil {
			entry = &aggregate{
				Path:   dir,
				Tokens: make(map[string]struct{}),
			}
			byDir[dir] = entry
		}
		entry.TotalScore += result.Score
		entry.MatchCount++
		entry.TopMatches = append(entry.TopMatches, result)

		for _, match := range result.Matches {
			token := tokenFromMatch(match)
			if token != "" {
				entry.Tokens[token] = struct{}{}
			}
		}
	}

	summaries := make([]DirectorySummary, 0, len(byDir))
	for _, entry := range byDir {
		sort.Slice(entry.TopMatches, func(i, j int) bool {
			if entry.TopMatches[i].Score != entry.TopMatches[j].Score {
				return entry.TopMatches[i].Score > entry.TopMatches[j].Score
			}
			return entry.TopMatches[i].Path < entry.TopMatches[j].Path
		})
		if len(entry.TopMatches) > drillLimit {
			entry.TopMatches = entry.TopMatches[:drillLimit]
		}

		tokens := make([]string, 0, len(entry.Tokens))
		for token := range entry.Tokens {
			tokens = append(tokens, token)
		}
		sort.Strings(tokens)
		if len(tokens) > 3 {
			tokens = tokens[:3]
		}

		rationale := fmt.Sprintf("matched %d result(s), total score %d", entry.MatchCount, entry.TotalScore)
		if len(tokens) > 0 {
			rationale += fmt.Sprintf(", top terms: %s", strings.Join(tokens, ", "))
		}

		summaries = append(summaries, DirectorySummary{
			Path:       entry.Path,
			TotalScore: entry.TotalScore,
			MatchCount: entry.MatchCount,
			Rationale:  rationale,
			TopMatches: entry.TopMatches,
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].TotalScore != summaries[j].TotalScore {
			return summaries[i].TotalScore > summaries[j].TotalScore
		}
		if summaries[i].MatchCount != summaries[j].MatchCount {
			return summaries[i].MatchCount > summaries[j].MatchCount
		}
		return summaries[i].Path < summaries[j].Path
	})
	if len(summaries) > dirLimit {
		summaries = summaries[:dirLimit]
	}
	return summaries
}

func directoryForResultPath(result SearchResult) string {
	path := filepath.ToSlash(result.Path)
	if result.Type == "directory" {
		return path
	}
	dir := filepath.ToSlash(filepath.Dir(path))
	if dir == "." {
		return "."
	}
	return dir
}

func tokenFromMatch(match string) string {
	parts := strings.SplitN(match, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func printDirectorySummaries(summaries []DirectorySummary) {
	if len(summaries) == 0 {
		fmt.Println("No directory summaries found")
		return
	}

	for i, summary := range summaries {
		fmt.Printf("%d. %s score=%d hits=%d\n", i+1, summary.Path, summary.TotalScore, summary.MatchCount)
		fmt.Printf("   rationale=%s\n", summary.Rationale)
		for _, hit := range summary.TopMatches {
			fmt.Printf("   - %s (%s) score=%d\n", hit.Path, hit.Type, hit.Score)
		}
	}
}

func directorySummariesToJSON(summaries []DirectorySummary) (string, error) {
	bytes, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
