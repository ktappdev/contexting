package contexting

import (
	"context"
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
	if _, err := GenerateSynonymsBatch([]string{"src"}, "", defaultModel, "", 0, 0, 4); err != nil {
		// empty API key is allowed now, but HTTP call will fail
		// just verify it doesn't panic
	}

	resp, err := GenerateSynonymsBatch(nil, "sk-test", defaultModel, "", 0, 0, 4)
	if err != nil {
		t.Fatalf("expected no error for empty names, got %v", err)
	}
	if len(resp) != 0 {
		t.Fatalf("expected empty response for empty names, got %d entries", len(resp))
	}

	// Test with symbols map (nil should work) - use empty names to avoid HTTP call
	resp2, err := GenerateSynonymsBatchWithContext(context.Background(), []string{}, "sk-test", defaultModel, "", 0, 0, 4, 4, nil)
	if err != nil {
		t.Fatalf("expected no error for empty names with nil symbols, got %v", err)
	}
	if len(resp2) != 0 {
		t.Fatalf("expected empty response for empty names with nil symbols, got %d entries", len(resp2))
	}
}

func TestGenerateSynonymsForNamesHandlesEmptyList(t *testing.T) {
	result, err := GenerateSynonymsForNames([]string{}, "sk-test", 8, defaultModel, "", 0, 0, 4, 4)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil map")
	}
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d entries", len(result))
	}

	// Test with symbols map
	result2, err := GenerateSynonymsForNamesWithContext(context.Background(), []string{}, "sk-test", 8, defaultModel, "", 0, 0, 4, 4, 1, nil)
	if err != nil {
		t.Fatalf("expected no error with nil symbols, got %v", err)
	}
	if result2 == nil {
		t.Fatalf("expected non-nil map with nil symbols")
	}
	if len(result2) != 0 {
		t.Fatalf("expected empty result with nil symbols, got %d entries", len(result2))
	}
}

func TestOpenRouterConstants(t *testing.T) {
	if defaultEndpoint != "https://openrouter.ai/api/v1/chat/completions" {
		t.Fatalf("unexpected defaultEndpoint: %s", defaultEndpoint)
	}
	if defaultModel != "deepseek/deepseek-v4-flash" {
		t.Fatalf("unexpected defaultModel: %s", defaultModel)
	}
	if defaultSynonyms != 5 {
		t.Fatalf("unexpected defaultSynonyms: %d", defaultSynonyms)
	}
}
