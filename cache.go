package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func LoadSynonymCache(path string) (SynonymResponse, error) {
	if path == "" {
		return make(SynonymResponse), nil
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(SynonymResponse), nil
		}
		return nil, fmt.Errorf("read synonym cache: %w", err)
	}

	cache := make(SynonymResponse)
	if err := json.Unmarshal(bytes, &cache); err != nil {
		return nil, fmt.Errorf("parse synonym cache: %w", err)
	}

	for name, syns := range cache {
		cache[name] = sanitizeSynonyms(syns, 16)
	}
	return cache, nil
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
	if err := writeFileAtomic(path, append(bytes, '\n'), 0o644); err != nil {
		return fmt.Errorf("write synonym cache: %w", err)
	}
	return nil
}
