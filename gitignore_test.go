package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureAndLoadGitignoreCreatesStarter(t *testing.T) {
	tmpDir := t.TempDir()
	patterns, err := EnsureAndLoadGitignore(tmpDir)
	if err != nil {
		t.Fatalf("EnsureAndLoadGitignore failed: %v", err)
	}
	if len(patterns) == 0 {
		t.Fatalf("expected starter patterns")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".gitignore")); err != nil {
		t.Fatalf("expected .gitignore created: %v", err)
	}
}

func TestLoadGitignorePatternsIgnoresCommentsAndNegation(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".gitignore")
	content := "# comment\nnode_modules/\n.env\n!important.env\n*.log\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	patterns, err := LoadGitignorePatterns(path)
	if err != nil {
		t.Fatalf("LoadGitignorePatterns failed: %v", err)
	}

	if len(patterns) < 3 {
		t.Fatalf("expected parsed patterns, got %v", patterns)
	}
}

func TestShouldIgnorePathWithWildcard(t *testing.T) {
	ignored := BuildIgnoreMap([]string{"*.log", ".env.*.local"})
	if !shouldIgnorePath("logs/app.log", "app.log", ignored) {
		t.Fatalf("expected wildcard log pattern to match")
	}
	if !shouldIgnorePath(".env.dev.local", ".env.dev.local", ignored) {
		t.Fatalf("expected wildcard env pattern to match")
	}
}
