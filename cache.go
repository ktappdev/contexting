package contexting

import (
	"encoding/json"
	"fmt"
	"os"
)

const currentCacheVersion = 2

type synonymCacheState struct {
	Version      int               `json:"cache_version"`
	Entries      SynonymResponse   `json:"entries"`
	Fingerprints map[string]string `json:"fingerprints,omitempty"`
}

func LoadSynonymCache(path string) (SynonymResponse, error) {
	state, err := loadSynonymCacheState(path)
	return state.Entries, err
}

func loadSynonymCacheState(path string) (synonymCacheState, error) {
	state := synonymCacheState{Version: currentCacheVersion, Entries: make(SynonymResponse), Fingerprints: make(map[string]string)}
	if path == "" {
		return state, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, fmt.Errorf("read synonym cache: %w", err)
	}
	// Decode into a fresh value so a missing version is rejected.
	var decoded synonymCacheState
	if err := json.Unmarshal(data, &decoded); err != nil {
		return state, fmt.Errorf("parse synonym cache: %w; remove the cache file and run ctxt init", err)
	}
	if decoded.Version != currentCacheVersion {
		return state, fmt.Errorf("unsupported synonym cache version %d; remove the cache file and run ctxt init", decoded.Version)
	}
	state = decoded

	if state.Entries == nil {
		state.Entries = make(SynonymResponse)
	}
	if state.Fingerprints == nil {
		state.Fingerprints = make(map[string]string)
	}
	for name, syns := range state.Entries {
		state.Entries[name] = sanitizeSynonyms(syns, 16)
	}
	return state, nil
}

func saveSynonymCacheState(path string, state synonymCacheState) error {
	if path == "" {
		return nil
	}
	state.Version = currentCacheVersion
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal synonym cache: %w", err)
	}
	return writeFileAtomic(path, append(data, '\n'), 0o600)
}

func SaveSynonymCache(path string, cache SynonymResponse) error {
	if cache == nil {
		cache = make(SynonymResponse)
	}
	return saveSynonymCacheState(path, synonymCacheState{Entries: cache})
}
