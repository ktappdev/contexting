package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

type TestConfig struct {
	App struct {
		Name    string
		Version string
	}
	Server struct {
		Port int
		Host string
	}
	Ignore []string
}

func TestTOMLDecode(t *testing.T) {
	configStr := `
Ignore = [".git", "node_modules", "vendor"]

[App]
Name = "test-app"
Version = "1.0.0"

[Server]
Port = 8080
Host = "localhost"
`

	var config TestConfig
	metadata, err := toml.Decode(configStr, &config)
	if err != nil {
		t.Fatalf("Failed to decode TOML: %v", err)
	}

	t.Logf("Keys: %v", metadata.Keys())
	t.Logf("Undecoded: %v", metadata.Undecoded())

	if config.App.Name != "test-app" {
		t.Errorf("Expected App.Name to be 'test-app', got '%s'", config.App.Name)
	}
	if config.App.Version != "1.0.0" {
		t.Errorf("Expected App.Version to be '1.0.0', got '%s'", config.App.Version)
	}
	if config.Server.Port != 8080 {
		t.Errorf("Expected Server.Port to be 8080, got %d", config.Server.Port)
	}
	if config.Server.Host != "localhost" {
		t.Errorf("Expected Server.Host to be 'localhost', got '%s'", config.Server.Host)
	}
	if len(config.Ignore) != 3 {
		t.Errorf("Expected 3 ignore patterns, got %d", len(config.Ignore))
	}
	if len(config.Ignore) > 0 && config.Ignore[0] != ".git" {
		t.Errorf("Expected first ignore pattern to be '.git', got '%s'", config.Ignore[0])
	}
}

func TestTOMLDecodeFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	configStr := `
[App]
Name = "my-app"
Version = "2.0.0"

[Server]
Port = 3000
Host = "0.0.0.0"

Ignore = ["*.log", "*.tmp"]
`

	configPath := filepath.Join(tmpDir, "config.toml")
	err := os.WriteFile(configPath, []byte(configStr), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	var config TestConfig
	_, err = toml.DecodeFile(configPath, &config)
	if err != nil {
		t.Fatalf("Failed to decode TOML file: %v", err)
	}

	if config.App.Name != "my-app" {
		t.Errorf("Expected App.Name to be 'my-app', got '%s'", config.App.Name)
	}
	if config.Server.Port != 3000 {
		t.Errorf("Expected Server.Port to be 3000, got %d", config.Server.Port)
	}
}

func TestTOMLEncode(t *testing.T) {
	var config TestConfig
	config.App.Name = "encoded-app"
	config.App.Version = "1.0.0"
	config.Server.Port = 9000
	config.Server.Host = "127.0.0.1"
	config.Ignore = []string{".gitignore", "*.swp"}

	var encoded bytes.Buffer
	encoder := toml.NewEncoder(&encoded)
	err := encoder.Encode(config)
	if err != nil {
		t.Fatalf("Failed to encode TOML: %v", err)
	}

	encodedStr := encoded.String()
	if len(encodedStr) == 0 {
		t.Error("Expected encoded string to not be empty")
	}

	var decoded TestConfig
	_, err = toml.Decode(encodedStr, &decoded)
	if err != nil {
		t.Fatalf("Failed to decode encoded TOML: %v", err)
	}

	if decoded.App.Name != config.App.Name {
		t.Errorf("Expected decoded App.Name to be '%s', got '%s'", config.App.Name, decoded.App.Name)
	}
	if decoded.Server.Port != config.Server.Port {
		t.Errorf("Expected decoded Server.Port to be %d, got %d", config.Server.Port, decoded.Server.Port)
	}
}

func TestTOMLDecodeEmpty(t *testing.T) {
	var config TestConfig
	_, err := toml.Decode("", &config)
	if err != nil {
		t.Fatalf("Failed to decode empty TOML: %v", err)
	}

	if config.App.Name != "" {
		t.Errorf("Expected empty App.Name, got '%s'", config.App.Name)
	}
}

func TestTOMLDecodeInvalid(t *testing.T) {
	var config TestConfig
	_, err := toml.Decode("invalid toml [[[", &config)
	if err == nil {
		t.Error("Expected error for invalid TOML, got nil")
	}
}
