package main

import (
	"os"
	"testing"
)

func TestGetAPIKey(t *testing.T) {
	t.Run("returns key when set", func(t *testing.T) {
		if err := os.Setenv("OPENROUTER_API_KEY", "sk-test-key"); err != nil {
			t.Fatalf("setenv failed: %v", err)
		}
		defer os.Unsetenv("OPENROUTER_API_KEY")

		key, err := GetAPIKey()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if key != "sk-test-key" {
			t.Fatalf("expected key sk-test-key, got %s", key)
		}
	})

	t.Run("returns error when not set", func(t *testing.T) {
		os.Unsetenv("OPENROUTER_API_KEY")
		if _, err := GetAPIKey(); err == nil {
			t.Fatalf("expected error when env var is missing")
		}
	})
}

func TestGenerateSynonymsBatchValidation(t *testing.T) {
	if _, err := GenerateSynonymsBatch([]string{"src"}, "", defaultModel, 4); err == nil {
		t.Fatalf("expected error for empty API key")
	}

	resp, err := GenerateSynonymsBatch(nil, "sk-test", defaultModel, 4)
	if err != nil {
		t.Fatalf("expected no error for empty names, got %v", err)
	}
	if len(resp) != 0 {
		t.Fatalf("expected empty response for empty names, got %d entries", len(resp))
	}
}

func TestGenerateSynonymsForNamesHandlesEmptyList(t *testing.T) {
	result, err := GenerateSynonymsForNames([]string{}, "sk-test", 8, defaultModel, 4)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil map")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d entries", len(result))
	}
}

func TestOpenRouterConstants(t *testing.T) {
	if openRouterURL != "https://openrouter.ai/api/v1/chat/completions" {
		t.Fatalf("unexpected openRouterURL: %s", openRouterURL)
	}
	if defaultModel != "openai/gpt-oss-safeguard-20b" {
		t.Fatalf("unexpected defaultModel: %s", defaultModel)
	}
}
