package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadRuntimeState(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".contexting_runtime.json")
	state := RuntimeState{
		RootPath:  tmpDir,
		Address:   "127.0.0.1:12345",
		PID:       42,
		StartedAt: time.Now().UTC().Round(time.Second),
	}

	if err := SaveRuntimeState(path, state); err != nil {
		t.Fatalf("save runtime state: %v", err)
	}

	loaded, err := LoadRuntimeState(path)
	if err != nil {
		t.Fatalf("load runtime state: %v", err)
	}
	if loaded.RootPath != state.RootPath || loaded.Address != state.Address || loaded.PID != state.PID {
		t.Fatalf("runtime state mismatch: got %+v want %+v", loaded, state)
	}
}
