package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSearchHintsReturnsStrongMatches(t *testing.T) {
	root := t.TempDir()
	index := &ContextIndex{
		RootPath:    root,
		GeneratedAt: time.Now().UTC(),
		Tree: &Node{
			FullPath: root,
			Type:     "directory",
			Children: map[string]*Node{
				"config": {
					FullPath: filepath.Join(root, "config"),
					Type:     "directory",
					Children: map[string]*Node{
						"local_store.go": {
							FullPath: filepath.Join(root, "config", "local_store.go"),
							Type:     "file",
							Synonyms: []string{"local storage", "cache"},
							Children: map[string]*Node{},
						},
					},
				},
				"handlers": {
					FullPath: filepath.Join(root, "handlers"),
					Type:     "directory",
					Children: map[string]*Node{},
				},
			},
		},
	}

	results := SearchHintsWithOptions(index, "check local storage", SearchOptions{Limit: 3, MinScore: 1, IncludeDebug: true})
	if len(results) == 0 {
		t.Fatalf("expected results, got none")
	}
	if results[0].Path != filepath.Join("config", "local_store.go") {
		t.Fatalf("expected best match to be config/local_store.go, got %s", results[0].Path)
	}
	if results[0].Score <= 0 {
		t.Fatalf("expected positive score, got %d", results[0].Score)
	}
	if len(results[0].Breakdown) == 0 {
		t.Fatalf("expected breakdown info with IncludeDebug=true")
	}
}

func TestSearchHintsNoTokens(t *testing.T) {
	results := SearchHints(&ContextIndex{}, "   ", 5)
	if len(results) != 0 {
		t.Fatalf("expected no results for empty query, got %d", len(results))
	}
}

func TestSearchHintsTypeFilter(t *testing.T) {
	root := t.TempDir()
	index := &ContextIndex{
		RootPath: root,
		Tree: &Node{
			FullPath: root,
			Type:     "directory",
			Children: map[string]*Node{
				"config": {
					FullPath: filepath.Join(root, "config"),
					Type:     "directory",
					Synonyms: []string{"settings"},
					Children: map[string]*Node{
						"app.yaml": {
							FullPath: filepath.Join(root, "config", "app.yaml"),
							Type:     "file",
							Synonyms: []string{"settings"},
							Children: map[string]*Node{},
						},
					},
				},
			},
		},
	}

	results := SearchHintsWithOptions(index, "settings", SearchOptions{Limit: 5, TypeFilter: "dirs", MinScore: 1})
	if len(results) == 0 {
		t.Fatalf("expected at least one directory result")
	}
	for _, result := range results {
		if result.Type != "directory" {
			t.Fatalf("expected only directory results, got %v", results)
		}
	}
}

func TestTokenizeFiltersLowSignalWords(t *testing.T) {
	tokens := tokenize("how to do auth in the app")
	if len(tokens) == 0 {
		t.Fatalf("expected non-empty tokens")
	}
	for _, token := range tokens {
		if token == "to" || token == "do" || token == "in" || token == "the" {
			t.Fatalf("expected low-signal token to be filtered, got %v", tokens)
		}
	}
}
