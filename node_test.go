package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildTreeNestedAndIgnore(t *testing.T) {
	tmpDir := t.TempDir()

	mustMkdir(t, filepath.Join(tmpDir, "src", "handlers"))
	mustMkdir(t, filepath.Join(tmpDir, "node_modules", "leftpad"))
	mustMkdir(t, filepath.Join(tmpDir, ".venv", "bin"))
	mustMkdir(t, filepath.Join(tmpDir, ".venv", "lib", "python3.11", "site-packages", "fastapi"))
	mustWriteFile(t, filepath.Join(tmpDir, "src", "main.go"), "package main")
	mustWriteFile(t, filepath.Join(tmpDir, "src", "handlers", "router.go"), "package handlers")
	mustWriteFile(t, filepath.Join(tmpDir, "node_modules", "leftpad", "index.js"), "module.exports={}")
	mustWriteFile(t, filepath.Join(tmpDir, ".venv", "bin", "activate"), "#!/bin/sh")
	mustWriteFile(t, filepath.Join(tmpDir, ".venv", "lib", "python3.11", "site-packages", "fastapi", "__init__.py"), "")

	tree, err := BuildTree(tmpDir, BuildIgnoreMap(nil))
	if err != nil {
		t.Fatalf("BuildTree returned error: %v", err)
	}

	if _, ok := tree.Children["src"]; !ok {
		t.Fatalf("expected src directory to exist")
	}
	if _, ok := tree.Children["node_modules"]; ok {
		t.Fatalf("expected node_modules to be ignored")
	}
	if _, ok := tree.Children[".venv"]; ok {
		t.Fatalf("expected .venv to be ignored")
	}

	src := tree.Children["src"]
	if _, ok := src.Children["handlers"]; !ok {
		t.Fatalf("expected nested handlers directory")
	}
	if _, ok := src.Children["main.go"]; !ok {
		t.Fatalf("expected src/main.go to exist")
	}
}

func TestCollectNamesForLLMUnique(t *testing.T) {
	tmpDir := t.TempDir()

	mustMkdir(t, filepath.Join(tmpDir, "api"))
	mustMkdir(t, filepath.Join(tmpDir, "internal", "api"))
	mustWriteFile(t, filepath.Join(tmpDir, "api", "index.go"), "")
	mustWriteFile(t, filepath.Join(tmpDir, "internal", "api", "index.go"), "")

	tree, err := BuildTree(tmpDir, BuildIgnoreMap(nil))
	if err != nil {
		t.Fatalf("BuildTree returned error: %v", err)
	}

	names := CollectNamesForLLM(tree)
	if len(names) == 0 {
		t.Fatalf("expected names, got none")
	}

	apiCount := 0
	for _, name := range names {
		if name == "api" {
			apiCount++
		}
	}
	if apiCount != 1 {
		t.Fatalf("expected deduplicated name 'api' once, got %d", apiCount)
	}
}

func TestAssignSynonymsToTree(t *testing.T) {
	tmpDir := t.TempDir()

	mustMkdir(t, filepath.Join(tmpDir, "cmd"))
	mustMkdir(t, filepath.Join(tmpDir, "tools", "cmd"))

	tree, err := BuildTree(tmpDir, BuildIgnoreMap(nil))
	if err != nil {
		t.Fatalf("BuildTree returned error: %v", err)
	}

	AssignSynonymsToTree(tree, SynonymResponse{"cmd": {"command", "cli"}}, 4)

	first := tree.Children["cmd"]
	second := tree.Children["tools"].Children["cmd"]

	if len(first.Synonyms) == 0 {
		t.Fatalf("expected synonyms on first cmd node, got %v", first.Synonyms)
	}
	if len(second.Synonyms) == 0 {
		t.Fatalf("expected synonyms on second cmd node, got %v", second.Synonyms)
	}
}

func TestComputeStats(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdir(t, filepath.Join(tmpDir, "src"))
	mustWriteFile(t, filepath.Join(tmpDir, "src", "main.go"), "package main")

	tree, err := BuildTree(tmpDir, BuildIgnoreMap(nil))
	if err != nil {
		t.Fatalf("BuildTree returned error: %v", err)
	}
	AssignSynonymsToTree(tree, SynonymResponse{"src": {"source"}}, 4)

	stats := ComputeStats(tree)
	if stats.TotalNodes != 3 {
		t.Fatalf("expected 3 nodes (root, src, main.go), got %d", stats.TotalNodes)
	}
	if stats.TotalFiles != 1 {
		t.Fatalf("expected 1 file, got %d", stats.TotalFiles)
	}
	if stats.TotalDirs != 2 {
		t.Fatalf("expected 2 directories, got %d", stats.TotalDirs)
	}
	if stats.SynonymNodes != 2 {
		t.Fatalf("expected 2 nodes with synonyms, got %d", stats.SynonymNodes)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
