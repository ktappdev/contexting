package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestIndexManagerApplyAndFlush(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	mgr := NewIndexManager(IndexManagerOptions{
		RootPath:        tmpDir,
		OutputPath:      filepath.Join(tmpDir, "context.json"),
		CachePath:       filepath.Join(tmpDir, ".contexting_synonyms_cache.json"),
		IgnoredPaths:    BuildIgnoreMap(nil),
		Model:           defaultModel,
		BatchSize:       8,
		SynonymsPerName: 4,
		UseLLM:          false,
	})

	stats, err := mgr.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if stats.TotalNodes == 0 {
		t.Fatalf("expected nodes after bootstrap")
	}

	if flushed, err := mgr.FlushIfDirty(); err != nil {
		t.Fatalf("initial flush failed: %v", err)
	} else if !flushed {
		t.Fatalf("expected initial flush to write snapshot")
	}

	newFile := filepath.Join(tmpDir, "b.txt")
	if err := os.WriteFile(newFile, []byte("b"), 0o644); err != nil {
		t.Fatalf("write new file: %v", err)
	}

	apply, err := mgr.ApplyChanges(context.Background(), map[string]fsnotify.Op{"b.txt": fsnotify.Create})
	if err != nil {
		t.Fatalf("apply create failed: %v", err)
	}
	if !apply.Changed {
		t.Fatalf("expected create apply to mark changed")
	}

	if err := os.Remove(newFile); err != nil {
		t.Fatalf("remove new file: %v", err)
	}
	apply, err = mgr.ApplyChanges(context.Background(), map[string]fsnotify.Op{"b.txt": fsnotify.Remove})
	if err != nil {
		t.Fatalf("apply remove failed: %v", err)
	}
	if !apply.Changed {
		t.Fatalf("expected remove apply to mark changed")
	}

	if flushed, err := mgr.FlushIfDirty(); err != nil {
		t.Fatalf("final flush failed: %v", err)
	} else if !flushed {
		t.Fatalf("expected final flush to write snapshot")
	}
}
