package contexting

import (
	"encoding/json"
	"fmt"
	"os"
)

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
	state := synonymCacheState{Entries: make(SynonymResponse), Fingerprints: make(map[string]string)}
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
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return state, fmt.Errorf("parse synonym cache: %w", err)
	}
	if version, ok := fields["cache_version"]; ok && len(version) > 0 && version[0] != '[' {
		if err := json.Unmarshal(data, &state); err != nil {
			return state, fmt.Errorf("parse synonym cache: %w", err)
		}
		if state.Version != 2 {
			return state, fmt.Errorf("unsupported synonym cache version %d", state.Version)
		}
	} else if err := json.Unmarshal(data, &state.Entries); err != nil {
		return state, fmt.Errorf("parse synonym cache: %w", err)
	}
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
	state.Version = 2
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal synonym cache: %w", err)
	}
	return writeFileAtomic(path, append(data, '\n'), 0o600)
}

func SaveSynonymCache(path string, cache SynonymResponse) error {
	if path == "" {
		return nil
	}
	if cache == nil {
		cache = make(SynonymResponse)
	}
	bytes, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal synonym cache: %w", err)
	}
	if err := writeFileAtomic(path, append(bytes, '\n'), 0o600); err != nil {
		return fmt.Errorf("write synonym cache: %w", err)
	}
	return nil
}
