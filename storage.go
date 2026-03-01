package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type ContextIndex struct {
	RootPath    string    `json:"root_path"`
	GeneratedAt time.Time `json:"generated_at"`
	Model       string    `json:"model,omitempty"`
	Tree        *Node     `json:"tree"`
}

func SaveContextIndex(path string, index *ContextIndex) error {
	if index == nil {
		return fmt.Errorf("index is nil")
	}

	bytes, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	if err := writeFileAtomic(path, append(bytes, '\n'), 0o644); err != nil {
		return fmt.Errorf("write index file: %w", err)
	}

	return nil
}

func LoadContextIndex(path string) (*ContextIndex, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read index file: %w", err)
	}

	var index ContextIndex
	if err := json.Unmarshal(bytes, &index); err != nil {
		return nil, fmt.Errorf("parse index file: %w", err)
	}

	return &index, nil
}
