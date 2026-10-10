package contexting

import (
	"encoding/json"
	"fmt"
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
	Limit             int
	MinScore          int
	TypeFilter        string
	IncludeDebug      bool
	ContentFallback   bool   // enable content-based fallback via ripgrep
	ContentMatchScore int    // score for grep-only files (defaults to 1)
	ContentRoot       string // project root for rg (empty defaults to index.RootPath)
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
	tokenGroups := make(map[string][]string)
	for _, primary := range tokenizeBase(query) {
		for _, variant := range tokenize(primary) {
			tokenGroups[variant] = append(tokenGroups[variant], primary)
		}
	}
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
		covered := make(map[string]bool)
		matches := make([]string, 0)
		breakdown := make([]string, 0)

		for _, token := range tokens {
			if token == "" {
				continue
			}
			before := score
			if relLower == token || baseNameLower == token {
				score += 12
				matches = append(matches, "exact:"+token)
				breakdown = append(breakdown, "exact match +12: "+token)
			} else if strings.Contains(baseNameLower, token) {
				score += 7
				matches = append(matches, "basename:"+token)
				breakdown = append(breakdown, "basename contains +7: "+token)
			} else if hasSegmentPrefix(segments, token) {
				score += 5
				matches = append(matches, "segment-prefix:"+token)
				breakdown = append(breakdown, "segment prefix +5: "+token)
			} else if strings.Contains(relLower, token) {
				score += 4
				matches = append(matches, "path:"+token)
				breakdown = append(breakdown, "path contains +4: "+token)
			}

			synonymScore, synonymMatches, synonymBreakdown := scoreSynonymToken(node.Synonyms, token)
			symbolScore, symbolMatches, symbolBreakdown := scoreSymbolToken(node.Symbols, token)
			score += synonymScore + symbolScore
			matches = append(matches, synonymMatches...)
			matches = append(matches, symbolMatches...)
			breakdown = append(breakdown, synonymBreakdown...)
			breakdown = append(breakdown, symbolBreakdown...)
			if score > before {
				for _, primary := range tokenGroups[token] {
					covered[primary] = true
				}
			}
		}

		if len(covered) > 1 {
			bonus := 4 * (len(covered) - 1)
			score += bonus
			breakdown = append(breakdown, fmt.Sprintf("query coverage +%d: %d distinct terms", bonus, len(covered)))
		}

		// Exact basename match — query is the filename (with or without extension)
		stemLower := strings.ToLower(strings.TrimSuffix(baseNameLower, filepath.Ext(baseNameLower)))
		queryLower := strings.ToLower(query)
		if queryLower == baseNameLower || queryLower == stemLower {
			score += 15
			matches = append(matches, "exact-basename:"+baseNameLower)
			breakdown = append(breakdown, "exact basename +15: "+baseNameLower)
		}

		if node.Type == "file" && score > 0 {
			score += 1
			breakdown = append(breakdown, "file bias +1")
		}

		if score <= 0 || score < opts.MinScore {
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
	if len(results) > 1 {
		literalMatch := false
		for _, match := range results[0].Matches {
			if strings.HasPrefix(match, "exact-basename:") || strings.HasPrefix(match, "sym-exact:") {
				literalMatch = true
				break
			}
		}
		cutoff := len(results)
		for i := 1; i < len(results); i++ {
			// If score drops by more than half from previous result, cut here
			if results[i-1].Score > 0 && results[i].Score > 0 && results[i].Score*2 < results[i-1].Score {
				cutoff = i
				break
			}
		}
		// Conceptual queries benefit from alternatives; literal lookups can stop
		// at the first clear gap without padding the response to three results.
		if !literalMatch && cutoff < 3 && len(results) >= 3 {
			cutoff = 3
		}
		if cutoff < len(results) {
			results = results[:cutoff]
		}
	}

	if opts.ContentFallback && passesTypeFilter(&Node{Type: "file"}, opts.TypeFilter) {
		existing := make(map[string]bool, len(results))
		for _, result := range results {
			existing[filepath.Clean(result.Path)] = true
		}
		for _, result := range searchIndexedContent(index, tokens, opts) {
			if !existing[filepath.Clean(result.Path)] {
				existing[filepath.Clean(result.Path)] = true
				results = append(results, result)
			}
		}
		sort.Slice(results, func(i, j int) bool {
			if results[i].Score != results[j].Score {
				return results[i].Score > results[j].Score
			}
			return results[i].Path < results[j].Path
		})
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

func tokenizeBase(input string) []string {
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
	return filteredBase
}

func tokenize(input string) []string {
	filteredBase := tokenizeBase(input)
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

func searchResponseToJSON(resp SearchResponse) (string, error) {
	bytes, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
