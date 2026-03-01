package main

import (
	"path/filepath"
	"testing"
)

func TestLoadAndSaveSynonymCache(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "synonyms_cache.json")

	initial, err := LoadSynonymCache(path)
	if err != nil {
		t.Fatalf("load empty cache: %v", err)
	}
	if len(initial) != 0 {
		t.Fatalf("expected empty cache for missing file, got %v", initial)
	}

	cache := SynonymResponse{"config": {"settings", "file", "state"}}
	if err := SaveSynonymCache(path, cache); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	loaded, err := LoadSynonymCache(path)
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if len(loaded["config"]) == 0 {
		t.Fatalf("expected config synonyms in loaded cache")
	}
	for _, syn := range loaded["config"] {
		if syn == "file" {
			t.Fatalf("expected generic synonym to be filtered, got %v", loaded["config"])
		}
	}
}
