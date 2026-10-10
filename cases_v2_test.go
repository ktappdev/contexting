package contexting

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCasesRejectsBareArray(t *testing.T) {
	content := `[
		{"query": "test query", "expect_any": ["file1.go"]},
		{"query": "another query", "expect_any": ["file2.go"]}
	]`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cases.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	_, err := LoadEvalCases(path)
	if err == nil {
		t.Fatal("expected legacy bare array to be rejected")
	}
}

func TestLoadCasesV2Object(t *testing.T) {
	content := `{
		"version": 2,
		"categories": {
			"auth": {
				"description": "Authentication cases",
				"cases": [
					{"query": "login", "expect_any": ["auth/login.go"]},
					{"query": "logout", "expect_any": ["auth/logout.go"]}
				]
			},
			"config": {
				"description": "Configuration cases",
				"cases": [
					{"query": "settings", "expect_any": ["config/settings.go"]}
				]
			}
		}
	}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cases.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cases, err := LoadEvalCases(path)
	if err != nil {
		t.Fatalf("LoadEvalCases error: %v", err)
	}
	if len(cases) != 3 {
		t.Fatalf("expected 3 cases, got %d", len(cases))
	}
	// Categories should be sorted alphabetically: auth, config
	if cases[0].Category != "auth" {
		t.Fatalf("expected first category 'auth', got %s", cases[0].Category)
	}
	if cases[1].Category != "auth" {
		t.Fatalf("expected second category 'auth', got %s", cases[1].Category)
	}
	if cases[2].Category != "config" {
		t.Fatalf("expected third category 'config', got %s", cases[2].Category)
	}
	if cases[0].Query != "login" {
		t.Fatalf("unexpected first query: %s", cases[0].Query)
	}
	if cases[1].Query != "logout" {
		t.Fatalf("unexpected second query: %s", cases[1].Query)
	}
	if cases[2].Query != "settings" {
		t.Fatalf("unexpected third query: %s", cases[2].Query)
	}
}

func TestLoadCasesV2MissingVersion(t *testing.T) {
	content := `{
		"categories": {
			"auth": {
				"description": "Authentication cases",
				"cases": [
					{"query": "login", "expect_any": ["auth/login.go"]}
				]
			}
		}
	}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cases.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	_, err := LoadEvalCases(path)
	if err == nil {
		t.Fatalf("expected error for missing version, got nil")
	}
}

func TestLoadCasesV2EmptyCategories(t *testing.T) {
	content := `{
		"version": 2,
		"categories": {}
	}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cases.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cases, err := LoadEvalCases(path)
	if err != nil {
		t.Fatalf("LoadEvalCases error: %v", err)
	}
	if len(cases) != 0 {
		t.Fatalf("expected 0 cases, got %d", len(cases))
	}
}

func TestLoadEvalCasesVersion2(t *testing.T) {
	content := `{
		"version": 2,
		"categories": {
			"test": {
				"description": "Test cases",
				"cases": [
					{"query": "test", "expect_any": ["file.go"]}
				]
			}
		}
	}`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cases.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	cases, err := LoadEvalCases(path)
	if err != nil {
		t.Fatalf("LoadEvalCases error: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(cases))
	}
	if cases[0].Category != "test" {
		t.Fatalf("expected category 'test', got %s", cases[0].Category)
	}
}
