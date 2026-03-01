package main

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

type SearchResult struct {
	Path      string   `json:"path"`
	Type      string   `json:"type"`
	Score     int      `json:"score"`
	Matches   []string `json:"matches"`
	Breakdown []string `json:"breakdown,omitempty"`
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
				if strings.Contains(synLower, token) || strings.Contains(token, synLower) {
					score += 5
					matches = append(matches, "syn:"+syn)
					breakdown = append(breakdown, "syn overlap +5: "+syn)
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
	expanded := make([]string, 0, len(base)*2)
	for _, token := range base {
		expanded = append(expanded, token)
		if strings.HasSuffix(token, "s") && len(token) > 3 {
			expanded = append(expanded, strings.TrimSuffix(token, "s"))
		}
		if strings.HasSuffix(token, "ing") && len(token) > 5 {
			expanded = append(expanded, strings.TrimSuffix(token, "ing"))
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
