package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIndexUsesCacheAndLexicalSynonyms(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "config"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "config", "localStore.go"), []byte("package config"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	result, err := BuildIndex(BuildOptions{
		RootPath:        tmpDir,
		IgnoredPaths:    BuildIgnoreMap(nil),
		SynonymsPerName: 4,
		SynonymCache: SynonymResponse{
			"config": {"settings", "file", "state"},
		},
	})
	if err != nil {
		t.Fatalf("BuildIndex error: %v", err)
	}

	configNode := result.Index.Tree.Children["config"]
	if configNode == nil {
		t.Fatalf("expected config node")
	}
	if len(configNode.Synonyms) == 0 {
		t.Fatalf("expected synonyms on config node")
	}

	foundSettings := false
	for _, syn := range configNode.Synonyms {
		if syn == "settings" {
			foundSettings = true
		}
		if syn == "file" {
			t.Fatalf("expected filtered synonyms, got %v", configNode.Synonyms)
		}
	}
	if !foundSettings {
		t.Fatalf("expected cached synonym 'settings' in %v", configNode.Synonyms)
	}

	fileNode := configNode.Children["localStore.go"]
	if fileNode == nil {
		t.Fatalf("expected localStore.go node")
	}
	if len(fileNode.Synonyms) == 0 {
		t.Fatalf("expected lexical synonyms on localStore.go")
	}
}
