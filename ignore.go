package main

import (
	"path/filepath"
	"strings"
)

const dotWhitelistKey = "__dot_whitelist__"

var defaultDotWhitelist = map[string]bool{
	".prettierrc":     true,
	".eslintrc":       true,
	".editorconfig":   true,
	".env.example":    true,
	".env.sample":     true,
	".env.template":   true,
	".npmrc":          true,
	".nvmrc":          true,
	".node-version":   true,
	".python-version": true,
	".ruby-version":   true,
	".tool-versions":  true,
	".dockerignore":   true,
	".stylelintrc":    true,
	".babelrc":        true,
	".postcssrc":      true,
	".lintstagedrc":   true,
	".huskyrc":        true,
	".gitattributes":  true,
	".npmignore":      true,
	".browserslistrc": true,
	".commitlintrc":   true,
}

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

	// Database migrations (boilerplate, low search value)
	"migrations",
	"pb_migrations",    // PocketBase
	"db/migrate",       // Rails
	"alembic",          // Python/Alembic
	"flyway",           // Java/Flyway

	// IDE
	".vscode",
	".idea",

	// OS junk
	".DS_Store",
	"Thumbs.db",
}

func BuildDotWhitelist(extra []string) map[string]bool {
	merged := make(map[string]bool, len(defaultDotWhitelist)+len(extra))
	for k, v := range defaultDotWhitelist {
		merged[k] = v
	}
	for _, name := range extra {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			merged[trimmed] = true
		}
	}
	return merged
}

// EmbedDotWhitelist stores the dot file whitelist in the ignore map via a sentinel key.
func EmbedDotWhitelist(ignored map[string]bool, whitelist map[string]bool) {
	for name := range whitelist {
		ignored[dotWhitelistKey+name] = true
	}
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

	// Check if baseName is a whitelisted dot file
	if strings.HasPrefix(baseName, ".") {
		if defaultDotWhitelist[baseName] || ignored[dotWhitelistKey+baseName] {
			return false
		}
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
