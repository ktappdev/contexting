package main

import (
	"os"
	"testing"
)

func TestGetAPIKey(t *testing.T) {
	t.Run("returns key when set", func(t *testing.T) {
		os.Setenv("OPENROUTER_API_KEY", "sk-test-key")
		defer os.Unsetenv("OPENROUTER_API_KEY")

		key, err := GetAPIKey()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if key != "sk-test-key" {
			t.Errorf("Expected key to be sk-test-key, got %s", key)
		}
	})

	t.Run("returns error when not set", func(t *testing.T) {
		os.Unsetenv("OPENROUTER_API_KEY")

		_, err := GetAPIKey()
		if err == nil {
			t.Error("Expected error when API key not set")
		}
	})
}

func TestGenerateSynonymsBatch(t *testing.T) {
	t.Run("returns error for empty API key", func(t *testing.T) {
		_, err := GenerateSynonymsBatch([]string{"src", "lib"}, "", "openrouter/free")
		if err == nil {
			t.Error("Expected error for empty API key")
		}
	})

	t.Run("returns error for empty names", func(t *testing.T) {
		_, err := GenerateSynonymsBatch([]string{}, "sk-test-key", "openrouter/free")
		if err != nil {
			t.Errorf("Expected no error for empty names, got %v", err)
		}
	})
}

func TestGenerateSynonymsForNames(t *testing.T) {
	t.Run("handles empty names list", func(t *testing.T) {
		result, err := GenerateSynonymsForNames([]string{}, "sk-test-key", 5, "openrouter/free")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result == nil {
			t.Error("Expected result to not be nil")
		}
		if len(result) != 0 {
			t.Errorf("Expected empty result, got %d items", len(result))
		}
	})
}

func TestOpenRouterURLConstant(t *testing.T) {
	if openRouterURL != "https://openrouter.ai/api/v1/chat/completions" {
		t.Errorf("Expected openRouterURL to be https://openrouter.ai/api/v1/chat/completions, got %s", openRouterURL)
	}
}

func TestDefaultModelConstant(t *testing.T) {
	if defaultModel != "openrouter/free" {
		t.Errorf("Expected defaultModel to be openrouter/free, got %s", defaultModel)
	}
}
