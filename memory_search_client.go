package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func QueryMemorySearch(runtimeFile string, query string, opts SearchOptions, expectedRoot string) ([]SearchResult, error) {
	state, err := LoadRuntimeState(runtimeFile)
	if err != nil {
		return nil, err
	}

	if state.RootPath == "" {
		return nil, fmt.Errorf("runtime state missing root_path: restart watch server in the project directory")
	}
	if state.RootPath != expectedRoot {
		return nil, fmt.Errorf("runtime state root path mismatch: expected %s, got %s. Use --root to specify the project directory or run from the project root", expectedRoot, state.RootPath)
	}

	reqBody, err := json.Marshal(memorySearchRequest{Query: query, Opts: opts})
	if err != nil {
		return nil, fmt.Errorf("marshal memory search request: %w", err)
	}

	url := fmt.Sprintf("http://%s/search", state.Address)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create memory search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query memory search server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("memory search server returned status %d", resp.StatusCode)
	}

	var payload memorySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode memory search response: %w", err)
	}
	return payload.Results, nil
}
