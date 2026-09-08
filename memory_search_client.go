package contexting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Client-side timeouts
const defaultClientTimeout = 2 * time.Second // Timeout for memory search HTTP client requests

func QueryMemorySearch(runtimeFile string, query string, opts SearchOptions, expectedRoot string) (memorySearchResponse, error) {
	state, err := LoadRuntimeState(runtimeFile)
	if err != nil {
		return memorySearchResponse{}, err
	}

	if state.RootPath == "" {
		return memorySearchResponse{}, fmt.Errorf("runtime state missing root_path: restart watch server in the project directory")
	}
	if state.RootPath != expectedRoot {
		return memorySearchResponse{}, fmt.Errorf("runtime state root path mismatch: expected %s, got %s. Use --root to specify the project directory or run from the project root", expectedRoot, state.RootPath)
	}
	if err := validateRuntimeAddress(state.Address); err != nil {
		return memorySearchResponse{}, err
	}

	reqBody, err := json.Marshal(memorySearchRequest{Query: query, Opts: opts})
	if err != nil {
		return memorySearchResponse{}, fmt.Errorf("marshal memory search request: %w", err)
	}

	url := fmt.Sprintf("http://%s/search", state.Address)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return memorySearchResponse{}, fmt.Errorf("create memory search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: defaultClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return memorySearchResponse{}, fmt.Errorf("query memory search server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return memorySearchResponse{}, fmt.Errorf("memory search server returned status %d", resp.StatusCode)
	}

	var payload memorySearchResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&payload); err != nil {
		return memorySearchResponse{}, fmt.Errorf("decode memory search response: %w", err)
	}
	return payload, nil
}

func validateRuntimeAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return fmt.Errorf("runtime state has invalid address: restart watch server")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("runtime state address is not loopback: restart watch server")
	}
	return nil
}
