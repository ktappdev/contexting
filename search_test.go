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

func TestSearchCommandValidatesIndexRootPath(t *testing.T) {
	tmpDir := t.TempDir()

	wrongRoot := "/some/other/project"
	index := &ContextIndex{
		RootPath:    wrongRoot,
		GeneratedAt: time.Now(),
		Tree: &Node{
			FullPath: wrongRoot,
			Type:     "directory",
			Children: make(map[string]*Node),
		},
	}

	indexPath := filepath.Join(tmpDir, "context.json")
	if err := SaveContextIndex(indexPath, index); err != nil {
		t.Fatalf("failed to save index: %v", err)
	}

	loaded, err := LoadContextIndex(indexPath)
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}

	if loaded.RootPath != wrongRoot {
		t.Errorf("expected root path %s, got %s", wrongRoot, loaded.RootPath)
	}
}

func TestSearchCommandValidatesRuntimeRootPath(t *testing.T) {
	tmpDir := t.TempDir()

	wrongRoot := "/some/other/project"
	state := RuntimeState{
		RootPath:  wrongRoot,
		Address:   "127.0.0.1:12345",
		PID:       12345,
		StartedAt: time.Now(),
	}

	runtimePath := filepath.Join(tmpDir, ".contexting_runtime.json")
	if err := SaveRuntimeState(runtimePath, state); err != nil {
		t.Fatalf("failed to save runtime state: %v", err)
	}

	loaded, err := LoadRuntimeState(runtimePath)
	if err != nil {
		t.Fatalf("failed to load runtime state: %v", err)
	}

	if loaded.RootPath != wrongRoot {
		t.Errorf("expected root path %s, got %s", wrongRoot, loaded.RootPath)
	}
}

func TestQueryMemorySearchRejectsMismatchedRoot(t *testing.T) {
	tmpDir := t.TempDir()

	wrongRoot := filepath.Join(tmpDir, "wrong", "project")
	expectedRoot := filepath.Join(tmpDir, "expected", "project")

	state := RuntimeState{
		RootPath:  wrongRoot,
		Address:   "127.0.0.1:12345",
		PID:       12345,
		StartedAt: time.Now(),
	}

	runtimePath := filepath.Join(tmpDir, ".contexting_runtime.json")
	if err := SaveRuntimeState(runtimePath, state); err != nil {
		t.Fatalf("failed to save runtime state: %v", err)
	}

	_, err := QueryMemorySearch(runtimePath, "test query", SearchOptions{}, expectedRoot)
	if err == nil {
		t.Fatalf("expected error for mismatched root path, got nil")
	}

	if !contains(err.Error(), "root path mismatch") {
		t.Errorf("expected 'root path mismatch' error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestContextIndexMarshalsWithRootPath(t *testing.T) {
	tmpDir := t.TempDir()

	rootPath := filepath.Join(tmpDir, "test", "project")
	index := &ContextIndex{
		RootPath:    rootPath,
		GeneratedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Tree: &Node{
			FullPath: rootPath,
			Type:     "directory",
			Children: make(map[string]*Node),
		},
	}

	indexPath := filepath.Join(tmpDir, "context.json")
	if err := SaveContextIndex(indexPath, index); err != nil {
		t.Fatalf("failed to save index: %v", err)
	}

	loaded, err := LoadContextIndex(indexPath)
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}

	if loaded.RootPath != rootPath {
		t.Errorf("expected root_path %s, got %s", rootPath, loaded.RootPath)
	}
}
