package main

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SearchResult struct {
	Path      string   `json:"path"`
	Type      string   `json:"type"`
	Score     int      `json:"score"`
	Matches   []string `json:"matches"`
	Breakdown []string `json:"breakdown,omitempty"`
}

type SearchResponse struct {
	Source           string         `json:"source"`
	IndexGeneratedAt *time.Time     `json:"index_generated_at,omitempty"`
	Fallback         *bool          `json:"fallback,omitempty"`
	Results          []SearchResult `json:"results"`
}

type SearchOptions struct {
	Limit        int
	MinScore     int
	TypeFilter   string
	IncludeDebug bool
}

func SearchHints(index *ContextIndex, query string, limit int) []SearchResult {
	return SearchHintsWithOptions(index, query, SearchOptions{Limit: limit})
}

func SearchHintsWithOptions(index *ContextIndex, query string, opts SearchOptions) []SearchResult {
	if index == nil || index.Tree == nil {
		return nil
	}
	if opts.Limit <= 0 {
		opts.Limit = 5
	}
	if opts.TypeFilter == "" {
		opts.TypeFilter = "all"
	}

	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil
	}

	results := make([]SearchResult, 0)
	root := index.Tree.FullPath

	walkTree(index.Tree, func(node *Node) {
		if node == index.Tree || !passesTypeFilter(node, opts.TypeFilter) {
			return
		}

		relPath := node.FullPath
		if rel, err := filepath.Rel(root, node.FullPath); err == nil {
			relPath = rel
		}
		relLower := strings.ToLower(filepath.ToSlash(relPath))
		baseNameLower := strings.ToLower(filepath.Base(node.FullPath))
		segments := strings.Split(relLower, "/")

		score := 0
		matches := make([]string, 0)
		breakdown := make([]string, 0)

		for _, token := range tokens {
			if token == "" {
				continue
			}
			if relLower == token || baseNameLower == token {
				score += 12
				matches = append(matches, "exact:"+token)
				breakdown = append(breakdown, "exact match +12: "+token)
			}

			if strings.Contains(baseNameLower, token) {
				score += 7
				matches = append(matches, "basename:"+token)
				breakdown = append(breakdown, "basename contains +7: "+token)
			}

			if strings.Contains(relLower, token) {
				score += 4
				matches = append(matches, "path:"+token)
				breakdown = append(breakdown, "path contains +4: "+token)
			}

			if hasSegmentPrefix(segments, token) {
				score += 5
				matches = append(matches, "segment-prefix:"+token)
				breakdown = append(breakdown, "segment prefix +5: "+token)
			}

			for _, syn := range node.Synonyms {
				synLower := strings.ToLower(syn)
				if synLower == "" {
					continue
				}
				if synLower == token {
					score += 8
					matches = append(matches, "syn-exact:"+syn)
					breakdown = append(breakdown, "syn exact +8: "+syn)
					continue
				}
				if len(token) < 3 || len(synLower) < 3 {
				continue
				}
				if strings.Contains(synLower, token) || strings.Contains(token, synLower) {
					score += 5
					matches = append(matches, "syn:"+syn)
					breakdown = append(breakdown, "syn overlap +5: "+syn)
				}
			}
		}

		// Exact basename match — query is the filename (with or without extension)
		stemLower := strings.ToLower(strings.TrimSuffix(baseNameLower, filepath.Ext(baseNameLower)))
		queryLower := strings.ToLower(query)
		if queryLower == baseNameLower || queryLower == stemLower {
			score += 15
			matches = append(matches, "exact-basename:"+baseNameLower)
			breakdown = append(breakdown, "exact basename +15: "+baseNameLower)
		}

		// Symbol scoring
		for _, sym := range node.Symbols {
			symLower := strings.ToLower(sym)
			symTokens := tokenizeIdentifier(symLower)
			for _, token := range tokens {
				if symLower == token {
					score += 8
					matches = append(matches, "sym-exact:"+sym)
					breakdown = append(breakdown, "sym exact +8: "+sym)
				} else if strings.Contains(symLower, token) {
					score += 5
					matches = append(matches, "sym:"+sym)
					breakdown = append(breakdown, "sym contains +5: "+sym)
				}
				for _, symToken := range symTokens {
					if symToken == token {
						score += 4
						matches = append(matches, "sym-token:"+symToken)
						breakdown = append(breakdown, "sym token +4: "+symToken)
					}
				}
			}
		}

		if node.Type == "file" && score > 0 {
			score += 1
			breakdown = append(breakdown, "file bias +1")
		}

		if score < opts.MinScore {
			return
		}

		result := SearchResult{
			Path:    relPath,
			Type:    node.Type,
			Score:   score,
			Matches: dedupeStrings(matches),
		}
		if opts.IncludeDebug {
			result.Breakdown = dedupeStrings(breakdown)
		}
		results = append(results, result)
	})

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Type != results[j].Type {
			return results[i].Type < results[j].Type
		}
		return results[i].Path < results[j].Path
	})

	// Apply confidence gap heuristic to reduce noise
	if len(results) > 3 {
		cutoff := len(results)
		for i := 1; i < len(results); i++ {
			// If score drops by more than half from previous result, cut here
			if results[i-1].Score > 0 && results[i].Score > 0 && results[i].Score*2 < results[i-1].Score {
				cutoff = i
				break
			}
		}
		// Don't cut below 3 results if scores are still positive
		if cutoff < 3 && len(results) >= 3 && results[2].Score > 0 {
			cutoff = 3
		}
		if cutoff < len(results) {
			results = results[:cutoff]
		}
	}
	if len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results
}

func hasSegmentPrefix(segments []string, token string) bool {
	for _, seg := range segments {
		if strings.HasPrefix(seg, token) {
			return true
		}
	}
	return false
}

func passesTypeFilter(node *Node, filter string) bool {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "", "all":
		return true
	case "files", "file":
		return node.Type == "file"
	case "dirs", "dir", "directories", "directory":
		return node.Type == "directory"
	default:
		return true
	}
}

func tokenize(input string) []string {
	cleaned := strings.ToLower(input)
	cleaned = strings.NewReplacer(
		",", " ",
		".", " ",
		"/", " ",
		"_", " ",
		"-", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		"\n", " ",
		"\t", " ",
	).Replace(cleaned)

	parts := strings.Fields(cleaned)
	if len(parts) == 0 {
		return nil
	}

	base := dedupeStrings(parts)
	filteredBase := make([]string, 0, len(base))
	for _, token := range base {
		if isLowSignalToken(token) || isSourceExtension(token) {
			continue
		}
		filteredBase = append(filteredBase, token)
	}
	if len(filteredBase) == 0 {
		return nil
	}
	expanded := make([]string, 0, len(filteredBase)*2)
	for _, token := range filteredBase {
		expanded = append(expanded, token)
		if strings.HasSuffix(token, "s") && len(token) > 3 {
			stem := strings.TrimSuffix(token, "s")
			if !isLowSignalToken(stem) {
				expanded = append(expanded, stem)
			}
		}
		if strings.HasSuffix(token, "ing") && len(token) > 5 {
			stem := strings.TrimSuffix(token, "ing")
			if !isLowSignalToken(stem) {
				expanded = append(expanded, stem)
			}
		}
	}
	return dedupeStrings(expanded)
}

func resultsToJSON(results []SearchResult) (string, error) {
	bytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func searchResponseToJSON(resp SearchResponse) (string, error) {
	bytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
