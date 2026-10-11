package contexting

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

const dotWhitelistKey = "__dot_whitelist__"
const gitignoreRulesKey = "__gitignore_rules__\n"

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
		normalized := strings.TrimPrefix(strings.TrimSpace(filepath.ToSlash(pattern)), "./")
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
	ignored := BuildIgnoreMap(extra)
	// Keep ordered Git rules separate from additive default/config exclusions.
	ignored[gitignoreRulesKey+strings.Join(patterns, "\n")] = true
	return ignored, nil
}

type ignoreMatcher struct {
	exclusions gitignore.Matcher
	gitRules   gitignore.Matcher
	whitelist  map[string]bool
}

func newIgnoreMatcher(ignored map[string]bool) ignoreMatcher {
	var exclusions []gitignore.Pattern
	var gitRules []gitignore.Pattern
	whitelist := BuildDotWhitelist(nil)
	keys := make([]string, 0, len(ignored))
	for key, enabled := range ignored {
		if enabled {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		switch {
		case strings.HasPrefix(key, dotWhitelistKey):
			whitelist[strings.TrimPrefix(key, dotWhitelistKey)] = true
		case strings.HasPrefix(key, gitignoreRulesKey):
			for _, line := range strings.Split(strings.TrimPrefix(key, gitignoreRulesKey), "\n") {
				gitRules = append(gitRules, gitignore.ParsePattern(line, nil))
			}
		default:
			// Config exclusions are additive; negation belongs in .gitignore.
			if !strings.HasPrefix(key, "!") {
				exclusions = append(exclusions, gitignore.ParsePattern(key, nil))
			}
		}
	}
	return ignoreMatcher{
		exclusions: gitignore.NewMatcher(exclusions),
		gitRules:   gitignore.NewMatcher(gitRules),
		whitelist:  whitelist,
	}
}

func (m ignoreMatcher) shouldIgnore(relPath, baseName string, isDir bool) bool {
	normalized := normalizeIgnorePattern(relPath)
	if normalized == "" {
		return false
	}
	segments := strings.Split(normalized, "/")
	for i, segment := range segments {
		last := i == len(segments)-1
		directory := !last || isDir
		// Whitelisting a dot file must never expose its hidden parent directory.
		if strings.HasPrefix(segment, ".") && (directory || !m.whitelist[baseName]) {
			return true
		}
		path := segments[:i+1]
		if m.exclusions.Match(path, directory) || m.gitRules.Match(path, directory) {
			return true
		}
	}
	return false
}

func shouldIgnorePath(relPath string, baseName string, ignored map[string]bool) bool {
	return newIgnoreMatcher(ignored).shouldIgnore(relPath, baseName, false)
}

func normalizeIgnorePattern(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	normalized := filepath.ToSlash(trimmed)
	return strings.Trim(normalized, "/")
}
