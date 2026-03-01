package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunDoctorHealthy(t *testing.T) {
	tmpDir := t.TempDir()

	index := &ContextIndex{
		RootPath:    tmpDir,
		GeneratedAt: time.Now().UTC(),
		Tree: &Node{
			FullPath: tmpDir,
			Type:     "directory",
			Children: map[string]*Node{},
		},
	}
	if err := SaveContextIndex(filepath.Join(tmpDir, "context.json"), index); err != nil {
		t.Fatalf("save index: %v", err)
	}
	if err := SaveSynonymCache(filepath.Join(tmpDir, ".contexting_synonyms_cache.json"), SynonymResponse{"src": {"code"}}); err != nil {
		t.Fatalf("save cache: %v", err)
	}
	if err := writeStarterConfig(filepath.Join(tmpDir, "context.toml"), false); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.Setenv("OPENROUTER_API_KEY", "sk-test"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	defer os.Unsetenv("OPENROUTER_API_KEY")

	report := RunDoctor(DoctorOptions{ConfigPath: filepath.Join(tmpDir, "context.toml"), RootPath: tmpDir, WriteCheck: true})
	if !report.Healthy {
		t.Fatalf("expected healthy report, got %+v", report)
	}
	if len(report.Checks) == 0 {
		t.Fatalf("expected checks in report")
	}
}

func TestRunDoctorFindsFailures(t *testing.T) {
	tmpDir := t.TempDir()
	report := RunDoctor(DoctorOptions{ConfigPath: filepath.Join(tmpDir, "missing.toml"), RootPath: filepath.Join(tmpDir, "missing-root"), WriteCheck: false})
	if report.Healthy {
		t.Fatalf("expected unhealthy report when root is missing")
	}
}
