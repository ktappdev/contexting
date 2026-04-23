package main

import (
	"path/filepath"
	"strings"
)

var defaultIgnores = []string{
	// Version control (ALWAYS ignore)
	".git",
	".svn",
	".hg",

	// Dependencies (ALWAYS ignore - can crash indexer)
	"node_modules",
	"vendor",
	"bower_components",

	// Virtual environments / caches
	".venv",
	".cache",
	".pytest_cache",
	"site-packages",
	"__pycache__",

	// Build outputs
	"dist",
	"build",
	"out",
	"tmp",
	"temp",

	// IDE
	".vscode",
	".idea",

	// OS junk
	".DS_Store",
	"Thumbs.db",
}

func BuildIgnoreMap(extra []string) map[string]bool {
	ignored := make(map[string]bool, len(defaultIgnores)+len(extra))
	for _, pattern := range defaultIgnores {
		ignored[normalizeIgnorePattern(pattern)] = true
	}
	for _, pattern := range extra {
		normalized := normalizeIgnorePattern(pattern)
		if normalized != "" {
			ignored[normalized] = true
		}
	}
	return ignored
}

func BuildIgnoreMapForRoot(root string, extra []string) (map[string]bool, error) {
	patterns, err := EnsureAndLoadGitignore(root)
	if err != nil {
		return nil, err
	}
	merged := make([]string, 0, len(patterns)+len(extra))
	merged = append(merged, patterns...)
	merged = append(merged, extra...)
	return BuildIgnoreMap(merged), nil
}

func shouldIgnorePath(relPath string, baseName string, ignored map[string]bool) bool {
	normalizedRel := normalizeIgnorePattern(relPath)
	if normalizedRel == "" {
		return false
	}

	if isDotPath(normalizedRel) || strings.HasPrefix(normalizeIgnorePattern(baseName), ".") {
		return true
	}

	if len(ignored) == 0 {
		return false
	}

	if ignored[normalizeIgnorePattern(baseName)] || ignored[normalizedRel] {
		return true
	}

	for pattern := range ignored {
		if !strings.Contains(pattern, "*") {
			continue
		}
		if matched, _ := filepath.Match(pattern, normalizeIgnorePattern(baseName)); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, normalizedRel); matched {
			return true
		}
	}

	segments := strings.Split(normalizedRel, "/")
	for _, segment := range segments {
		if ignored[segment] {
			return true
		}
	}

	return false
}

func normalizeIgnorePattern(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	normalized := filepath.ToSlash(trimmed)
	return strings.Trim(normalized, "/")
}

func isDotPath(normalizedRel string) bool {
	segments := strings.Split(normalizedRel, "/")
	for _, segment := range segments {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}
