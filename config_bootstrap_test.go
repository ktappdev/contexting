package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStarterConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "context.toml")

	if err := writeStarterConfig(path, false); err != nil {
		t.Fatalf("writeStarterConfig failed: %v", err)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if len(bytes) == 0 {
		t.Fatalf("expected non-empty starter config")
	}

	if err := writeStarterConfig(path, false); err == nil {
		t.Fatalf("expected error when writing existing config without force")
	}
	if err := writeStarterConfig(path, true); err != nil {
		t.Fatalf("expected overwrite with force, got: %v", err)
	}
}

func TestEnsureStarterConfigPromptAutoCreate(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "context.toml")

	if err := ensureStarterConfigPrompt(path, true); err != nil {
		t.Fatalf("ensureStarterConfigPrompt auto-create failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config to exist: %v", err)
	}
}
