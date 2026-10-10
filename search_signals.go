package contexting

import (
	"sort"
	"strings"
)

// Each query token receives only its strongest synonym match. Repeating a
// concept across synonyms must not increase a file's relevance.
func scoreSynonymToken(synonyms []string, token string) (int, []string, []string) {
	score := 0
	var matches, breakdown []string
	for _, synonym := range synonyms {
		lower := strings.ToLower(synonym)
		if lower == "" {
			continue
		}
		if lower == token {
			return 8, []string{"syn-exact:" + synonym}, []string{"syn exact +8: " + synonym}
		}
		if score == 0 && len(token) >= 3 && len(lower) >= 3 && (strings.Contains(lower, token) || strings.Contains(token, lower)) {
			score = 5
			matches = []string{"syn:" + synonym}
			breakdown = []string{"syn overlap +5: " + synonym}
		}
	}
	return score, matches, breakdown
}

// Keep at most three independent symbol matches per token. This preserves
// supporting evidence without allowing large files to accumulate unbounded scores.
func scoreSymbolToken(symbols []string, token string) (int, []string, []string) {
	type signal struct {
		score              int
		matches, breakdown []string
	}
	candidates := make([]signal, 0, 3)
	seen := make(map[string]bool)
	for _, symbol := range symbols {
		lower := strings.ToLower(symbol)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		score := 0
		var candidateMatches, candidateBreakdown []string
		if lower == token {
			score = 24
			candidateMatches = append(candidateMatches, "sym-exact:"+symbol)
			candidateBreakdown = append(candidateBreakdown, "sym exact +24: "+symbol)
		} else if strings.Contains(lower, token) {
			score = 5
			candidateMatches = append(candidateMatches, "sym:"+symbol)
			candidateBreakdown = append(candidateBreakdown, "sym contains +5: "+symbol)
		}
		for _, part := range tokenizeIdentifier(symbol) {
			if part == token {
				score += 4
				candidateMatches = append(candidateMatches, "sym-token:"+part)
				candidateBreakdown = append(candidateBreakdown, "sym token +4: "+part)
				break
			}
		}
		if score > 0 {
			candidates = append(candidates, signal{score, candidateMatches, candidateBreakdown})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	score := 0
	var matches, breakdown []string
	for _, candidate := range candidates {
		score += candidate.score
		matches = append(matches, candidate.matches...)
		breakdown = append(breakdown, candidate.breakdown...)
	}
	return score, matches, breakdown
}
