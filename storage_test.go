package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadContextIndex(t *testing.T) {
	tmpDir := t.TempDir()
	output := filepath.Join(tmpDir, "nested", "context.json")

	index := &ContextIndex{
		RootPath:    "/tmp/project",
		GeneratedAt: time.Now().UTC().Round(time.Second),
		Model:       defaultModel,
		Tree: &Node{
			FullPath: "/tmp/project",
			Type:     "directory",
			Children: map[string]*Node{
				"main.go": {
					FullPath: "/tmp/project/main.go",
					Type:     "file",
					Children: map[string]*Node{},
				},
			},
		},
	}

	if err := SaveContextIndex(output, index); err != nil {
		t.Fatalf("SaveContextIndex returned error: %v", err)
	}

	loaded, err := LoadContextIndex(output)
	if err != nil {
		t.Fatalf("LoadContextIndex returned error: %v", err)
	}

	if loaded.RootPath != index.RootPath {
		t.Fatalf("expected root path %s, got %s", index.RootPath, loaded.RootPath)
	}
	if loaded.Model != index.Model {
		t.Fatalf("expected model %s, got %s", index.Model, loaded.Model)
	}
	if loaded.Tree == nil || loaded.Tree.Children["main.go"] == nil {
		t.Fatalf("expected tree with main.go child")
	}
}
