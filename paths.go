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
	// Normalize: if the config file lives in .ctxt/ and the path already has a
	// .ctxt/ prefix, strip it to prevent double nesting (e.g. .ctxt/.ctxt/...).
	// This handles legacy configs where users put the full .ctxt/ prefix in
	// [search] or [eval] index/cases paths.
	configDir := filepath.Dir(configFile)
	if filepath.Base(configDir) == ".ctxt" {
		// Normalize separators and strip leading .ctxt/ or .ctxt\ prefix to
		// prevent double nesting (e.g. .ctxt/.ctxt/...). Uses filepath.Clean
		// and Split to handle both Unix and Windows path separators.
		// Normalize both / and \ first so paths work regardless of platform separator.
		// e.g. ".ctxt\\sub\\path" on Unix or ".ctxt/sub/path" on Windows.
		normalized := strings.ReplaceAll(path, "\\", string(filepath.Separator))
		normalized = strings.ReplaceAll(normalized, "/", string(filepath.Separator))
		cleaned := filepath.Clean(normalized)
		parts := strings.Split(cleaned, string(filepath.Separator))
		if len(parts) > 0 && parts[0] == ".ctxt" {
			path = filepath.Join(parts[1:]...)
		}
	}
	return filepath.Join(configDir, path)
}
