package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var starterGitignoreEntries = []string{
	"node_modules/",
	"vendor/",
	".env",
	".env.local",
	".env.*.local",
	"dist/",
	"build/",
	"tmp/",
	"*.log",
	".DS_Store",
}

func EnsureAndLoadGitignore(root string) ([]string, error) {
	gitignorePath := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(gitignorePath); err != nil {
		if os.IsNotExist(err) {
			if err := createStarterGitignore(gitignorePath); err != nil {
				return nil, err
			}
			logInfof("Created starter .gitignore at %s", gitignorePath)
		} else {
			return nil, fmt.Errorf("stat .gitignore: %w", err)
		}
	}

	patterns, err := LoadGitignorePatterns(gitignorePath)
	if err != nil {
		return nil, err
	}
	return patterns, nil
}

func createStarterGitignore(path string) error {
	lines := []string{"# Contexting starter .gitignore"}
	lines = append(lines, starterGitignoreEntries...)
	content := strings.Join(lines, "\n") + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create .gitignore directory: %w", err)
	}
	return writeFileAtomic(path, []byte(content), 0o644)
}

func LoadGitignorePatterns(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open .gitignore: %w", err)
	}
	defer file.Close()

	patterns := make([]string, 0, 32)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		line = strings.TrimPrefix(line, "./")
		line = strings.TrimPrefix(line, "/")
		line = strings.TrimSuffix(line, "/")
		if line == "" {
			continue
		}
		patterns = append(patterns, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read .gitignore: %w", err)
	}
	return dedupeStrings(patterns), nil
}
