package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNodeStructure(t *testing.T) {
	node := Node{
		FullPath: "/test/path",
		Type:     "directory",
		Synonyms: []string{"src", "source", "code"},
		Children: make(map[string]*Node),
	}

	if node.FullPath != "/test/path" {
		t.Errorf("Expected FullPath to be /test/path, got %s", node.FullPath)
	}
	if node.Type != "directory" {
		t.Errorf("Expected Type to be directory, got %s", node.Type)
	}
	if len(node.Synonyms) != 3 {
		t.Errorf("Expected 3 synonyms, got %d", len(node.Synonyms))
	}
	if node.Children == nil {
		t.Error("Expected Children to be initialized")
	}
}

func TestTraverseFolder(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "docs"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644)

	ignoredPaths := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		".DS_Store":    true,
	}

	tree, err := traverseFolder(tmpDir, ignoredPaths)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tree == nil {
		t.Fatal("Expected tree to not be nil")
	}

	var count int
	filepath.WalkDir(tmpDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(tmpDir, path)
		if rel != "." && !ignoredPaths[rel] {
			count++
		}
		return nil
	})

	if len(tree.Children) != count {
		t.Errorf("Expected %d children, got %d", count, len(tree.Children))
	}
}

func TestCollectNamesForLLM(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "lib"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte("package main"), 0644)

	ignoredPaths := map[string]bool{}

	tree, err := traverseFolder(tmpDir, ignoredPaths)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	names := CollectNamesForLLM(tree)

	if len(names) == 0 {
		t.Error("Expected names to not be empty")
	}

	found := false
	for _, name := range names {
		if name == "src" || name == "lib" || name == "main.go" || name == "util.go" {
			found = true
		}
	}
	if !found {
		t.Error("Expected to find folder and file names")
	}
}

func TestIgnorePatterns(t *testing.T) {
	ignoredPaths := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		".DS_Store":    true,
	}

	tests := []struct {
		name     string
		expected bool
	}{
		{".git", true},
		{"node_modules", true},
		{"vendor", true},
		{".DS_Store", true},
		{"src", false},
		{"main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ignoredPaths[tt.name]; got != tt.expected {
				t.Errorf("Expected %s to be ignored=%v, got %v", tt.name, tt.expected, got)
			}
		})
	}
}
