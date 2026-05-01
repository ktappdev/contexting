package main

import (
	"path/filepath"
	"strings"
)

func resolveProjectPath(projectRoot string, path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	if projectRoot == "" {
		return path
	}
	return filepath.Join(projectRoot, path)
}

func resolveConfigPath(configFile string, path string) string {
	if path == "" || filepath.IsAbs(path) || configFile == "" {
		return path
	}
	// Normalize: if the config file lives in .ctx/ and the path already has a
	// .ctx/ prefix, strip it to prevent double nesting (e.g. .ctx/.ctx/...).
	// This handles legacy configs where users put the full .ctx/ prefix in
	// [search] or [eval] index/cases paths.
	configDir := filepath.Dir(configFile)
	if filepath.Base(configDir) == ".ctx" {
		// Normalize separators and strip leading .ctx/ or .ctx\ prefix to
		// prevent double nesting (e.g. .ctx/.ctx/...). Uses filepath.Clean
		// and Split to handle both Unix and Windows path separators.
		// Normalize both / and \ first so paths work regardless of platform separator.
		// e.g. ".ctx\\sub\\path" on Unix or ".ctx/sub/path" on Windows.
		normalized := strings.ReplaceAll(path, "\\", string(filepath.Separator))
		normalized = strings.ReplaceAll(normalized, "/", string(filepath.Separator))
		cleaned := filepath.Clean(normalized)
		parts := strings.Split(cleaned, string(filepath.Separator))
		if len(parts) > 0 && parts[0] == ".ctx" {
			path = filepath.Join(parts[1:]...)
		}
	}
	return filepath.Join(configDir, path)
}
