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
		OutputPath:      filepath.Join(tmpDir, ".ctxt", "ctx_index.json"),
		CachePath:       filepath.Join(tmpDir, ".ctxt", "ctx_cache.json"),
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

func TestBootstrapStaleSnapshot(t *testing.T) {
	tmpDir := t.TempDir()

	// Step 1: Create initial files and bootstrap
	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write seed file a.txt: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "subdir"), 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "subdir", "c.txt"), []byte("charlie"), 0o644); err != nil {
		t.Fatalf("write seed file subdir/c.txt: %v", err)
	}

	mgr := NewIndexManager(IndexManagerOptions{
		RootPath:        tmpDir,
		OutputPath:      filepath.Join(tmpDir, ".ctxt", "ctx_index.json"),
		CachePath:       filepath.Join(tmpDir, ".ctxt", "ctx_cache.json"),
		IgnoredPaths:    BuildIgnoreMap(nil),
		Model:           defaultModel,
		BatchSize:       8,
		SynonymsPerName: 4,
		UseLLM:          false,
	})

	stats, err := mgr.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("first bootstrap failed: %v", err)
	}
	if stats.TotalFiles < 2 {
		t.Fatalf("expected at least 2 files after first bootstrap, got %d", stats.TotalFiles)
	}

	// Flush snapshot so we can load it on next bootstrap
	if flushed, err := mgr.FlushIfDirty(); err != nil {
		t.Fatalf("initial flush failed: %v", err)
	} else if !flushed {
		t.Fatalf("expected initial flush")
	}

	// Step 2: Make filesystem changes while watch is NOT running
	// Delete a.txt
	if err := os.Remove(filepath.Join(tmpDir, "a.txt")); err != nil {
		t.Fatalf("remove a.txt: %v", err)
	}
	// Add b.txt
	if err := os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("bravo"), 0o644); err != nil {
		t.Fatalf("write new file b.txt: %v", err)
	}
	// Modify c.txt
	if err := os.WriteFile(filepath.Join(tmpDir, "subdir", "c.txt"), []byte("charlie-updated"), 0o644); err != nil {
		t.Fatalf("modify subdir/c.txt: %v", err)
	}

	// Step 3: Bootstrap again — should detect changes via diff
	mgr2 := NewIndexManager(IndexManagerOptions{
		RootPath:        tmpDir,
		OutputPath:      filepath.Join(tmpDir, ".ctxt", "ctx_index.json"),
		CachePath:       filepath.Join(tmpDir, ".ctxt", "ctx_cache.json"),
		IgnoredPaths:    BuildIgnoreMap(nil),
		Model:           defaultModel,
		BatchSize:       8,
		SynonymsPerName: 4,
		UseLLM:          false,
	})

	stats2, err := mgr2.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("second bootstrap failed: %v", err)
	}

	// Step 4: Verify b.txt is in the index and a.txt is not
	foundB := false
	foundA := false
	walkTree(mgr2.index.Tree, func(node *Node) {
		if node == mgr2.index.Tree {
			return
		}
		base := filepath.Base(node.FullPath)
		if base == "b.txt" {
			foundB = true
		}
		if base == "a.txt" {
			foundA = true
		}
	})
	if !foundB {
		t.Fatalf("expected b.txt to be in index after stale snapshot diff")
	}
	if foundA {
		t.Fatalf("expected a.txt to be removed from index after stale snapshot diff")
	}

	// Step 5: Verify stats reflect updated state
	if stats2.TotalFiles < 2 {
		t.Fatalf("expected at least 2 files in updated stats, got %d", stats2.TotalFiles)
	}

	// Step 6: Verify snapshot was marked dirty (needs flush)
	if !mgr2.dirty {
		t.Fatalf("expected manager to be dirty after stale snapshot diff")
	}
}
