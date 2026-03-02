package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func syncWatchDirectories(watcher *fsnotify.Watcher, root string, ignored map[string]bool, watched map[string]struct{}) error {
	seen := make(map[string]struct{})

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel != "." && shouldIgnorePath(rel, d.Name(), ignored) {
			return filepath.SkipDir
		}

		seen[path] = struct{}{}
		if _, exists := watched[path]; !exists {
			if err := watcher.Add(path); err != nil {
				return fmt.Errorf("add watch %s: %w", path, err)
			}
			watched[path] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for dir := range watched {
		if _, exists := seen[dir]; exists {
			continue
		}
		_ = watcher.Remove(dir)
		delete(watched, dir)
	}

	return nil
}

func shouldSkipEvent(root string, event fsnotify.Event, ignored map[string]bool, outputPath string, cachePath string) bool {
	if shouldSkipInternalOutput(event.Name, outputPath, cachePath) {
		return true
	}
	rel, err := filepath.Rel(root, event.Name)
	if err != nil {
		return false
	}
	if rel == "." {
		return false
	}
	return shouldIgnorePath(rel, filepath.Base(event.Name), ignored)
}

func shouldSkipInternalOutput(eventPath string, outputPath string, cachePath string) bool {
	if eventPath == outputPath || eventPath == cachePath {
		return true
	}
	base := filepath.Base(eventPath)
	if matched, _ := filepath.Match(".tmp-*.json", base); matched {
		return true
	}
	return false
}

func logChangeSummary(changes map[string]fsnotify.Op) {
	if len(changes) == 0 {
		return
	}

	created := 0
	modified := 0
	removed := 0
	renamed := 0
	details := make([]string, 0, len(changes))

	paths := make([]string, 0, len(changes))
	for path := range changes {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		op := changes[path]
		if op&fsnotify.Create != 0 {
			created++
		}
		if op&fsnotify.Write != 0 || op&fsnotify.Chmod != 0 {
			modified++
		}
		if op&fsnotify.Remove != 0 {
			removed++
		}
		if op&fsnotify.Rename != 0 {
			renamed++
		}
		details = append(details, fmt.Sprintf("%s (%s)", path, summarizeOp(op)))
	}

	logInfof("Filesystem changes: created=%d modified=%d removed=%d renamed=%d", created, modified, removed, renamed)
	const maxDetails = 10
	if len(details) <= maxDetails {
		logInfof("Changed files: %s", strings.Join(details, ", "))
		return
	}
	logInfof("Changed files: %s, ... and %d more", strings.Join(details[:maxDetails], ", "), len(details)-maxDetails)
}

func summarizeOp(op fsnotify.Op) string {
	parts := make([]string, 0, 4)
	if op&fsnotify.Create != 0 {
		parts = append(parts, "create")
	}
	if op&fsnotify.Write != 0 {
		parts = append(parts, "write")
	}
	if op&fsnotify.Remove != 0 {
		parts = append(parts, "remove")
	}
	if op&fsnotify.Rename != 0 {
		parts = append(parts, "rename")
	}
	if op&fsnotify.Chmod != 0 {
		parts = append(parts, "chmod")
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, "|")
}

func parsePersistMode(value string) (WatchPersistMode, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch WatchPersistMode(normalized) {
	case PersistShutdown:
		return PersistShutdown, nil
	case PersistInterval:
		return PersistInterval, nil
	case PersistChange:
		return PersistChange, nil
	default:
		return "", fmt.Errorf("invalid persist mode %q (expected shutdown|interval|change)", value)
	}
}

func tickerChan(ticker *time.Ticker) <-chan time.Time {
	if ticker == nil {
		return nil
	}
	return ticker.C
}
