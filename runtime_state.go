package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type RuntimeState struct {
	RootPath  string    `json:"root_path"`
	Address   string    `json:"address"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

func SaveRuntimeState(path string, state RuntimeState) error {
	bytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runtime state: %w", err)
	}
	if err := writeFileAtomic(path, append(bytes, '\n'), 0o644); err != nil {
		return fmt.Errorf("write runtime state: %w", err)
	}
	return nil
}

func LoadRuntimeState(path string) (*RuntimeState, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read runtime state: %w", err)
	}
	var state RuntimeState
	if err := json.Unmarshal(bytes, &state); err != nil {
		return nil, fmt.Errorf("parse runtime state: %w", err)
	}
	return &state, nil
}
