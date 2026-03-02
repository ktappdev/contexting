package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type WatchPersistMode string

const (
	PersistShutdown WatchPersistMode = "shutdown"
	PersistInterval WatchPersistMode = "interval"
	PersistChange   WatchPersistMode = "change"
)

type IndexManagerOptions struct {
	RootPath        string
	OutputPath      string
	CachePath       string
	IgnoredPaths    map[string]bool
	Model           string
	BatchSize       int
	SynonymsPerName int
	APIKey          string
	UseLLM          bool
}

type ApplyResult struct {
	Stats        IndexStats
	SynonymError error
	Changed      bool
}

type IndexManager struct {
	mu sync.Mutex

	rootPath        string
	outputPath      string
	cachePath       string
	ignored         map[string]bool
	model           string
	batchSize       int
	synonymsPerName int
	apiKey          string
	useLLM          bool

	index  *ContextIndex
	cache  SynonymResponse
	dirty  bool
	loaded bool
}

func NewIndexManager(opts IndexManagerOptions) *IndexManager {
	cache := make(SynonymResponse)
	if opts.Model == "" {
		opts.Model = defaultModel
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 8
	}
	if opts.SynonymsPerName <= 0 {
		opts.SynonymsPerName = 4
	}
	if opts.IgnoredPaths == nil {
		opts.IgnoredPaths = BuildIgnoreMap(nil)
	}

	return &IndexManager{
		rootPath:        opts.RootPath,
		outputPath:      opts.OutputPath,
		cachePath:       opts.CachePath,
		ignored:         opts.IgnoredPaths,
		model:           opts.Model,
		batchSize:       opts.BatchSize,
		synonymsPerName: opts.SynonymsPerName,
		apiKey:          opts.APIKey,
		useLLM:          opts.UseLLM,
		cache:           cache,
	}
}

func (m *IndexManager) Bootstrap(ctx context.Context) (IndexStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cache, err := LoadSynonymCache(m.cachePath)
	if err != nil {
		return IndexStats{}, err
	}
	m.cache = cache

	loadedIndex, err := LoadContextIndex(m.outputPath)
	if err == nil && loadedIndex != nil && loadedIndex.Tree != nil {
		if absRoot, rootErr := filepath.Abs(m.rootPath); rootErr == nil && loadedIndex.RootPath == absRoot {
			m.index = loadedIndex
			m.loaded = true
			m.dirty = false
			stats := ComputeStats(m.index.Tree)
			stats.CollectedNames = len(CollectNamesForLLM(m.index.Tree))
			return stats, nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		logWarnf("Unable to load existing context snapshot: %v", err)
	}

	result, buildErr := BuildIndex(BuildOptions{
		Ctx:             ctx,
		RootPath:        m.rootPath,
		IgnoredPaths:    m.ignored,
		APIKey:          m.activeAPIKey(),
		Model:           m.model,
		BatchSize:       m.batchSize,
		SynonymsPerName: m.synonymsPerName,
		SynonymCache:    m.cache,
	})
	if buildErr != nil {
		return IndexStats{}, buildErr
	}
	m.index = result.Index
	m.cache = result.SynonymCache
	m.dirty = true
	m.loaded = true
	return result.Stats, nil
}

func (m *IndexManager) ApplyChanges(ctx context.Context, changes map[string]fsnotify.Op) (ApplyResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.loaded || m.index == nil || m.index.Tree == nil {
		return ApplyResult{}, fmt.Errorf("index manager not bootstrapped")
	}

	result := ApplyResult{}
	if len(changes) == 0 {
		result.Stats = ComputeStats(m.index.Tree)
		result.Stats.CollectedNames = len(CollectNamesForLLM(m.index.Tree))
		return result, nil
	}

	missingNames := make(map[string]struct{})
	paths := make([]string, 0, len(changes))
	for path := range changes {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, relPath := range paths {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		op := changes[relPath]
		absPath := filepath.Join(m.rootPath, filepath.FromSlash(relPath))
		baseName := filepath.Base(absPath)

		if op&(fsnotify.Remove|fsnotify.Rename) != 0 {
			if removeNodeByRelPath(m.index.Tree, relPath) {
				result.Changed = true
			}
		}

		if op&(fsnotify.Create|fsnotify.Write|fsnotify.Chmod) == 0 {
			continue
		}

		isDir, statErr := isExistingDirectory(absPath)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return result, statErr
		}

		if shouldIgnorePath(relPath, baseName, m.ignored) {
			_ = removeNodeByRelPath(m.index.Tree, relPath)
			continue
		}

		if upsertNodeByRelPath(m.index.Tree, m.rootPath, relPath, isDir, m.cache, m.synonymsPerName) {
			result.Changed = true
		}

		if m.useLLM {
			if _, ok := m.cache[baseName]; !ok {
				missingNames[baseName] = struct{}{}
			}
		}
	}

	if ctx.Err() != nil {
		return result, ctx.Err()
	}

	if m.useLLM && len(missingNames) > 0 {
		names := make([]string, 0, len(missingNames))
		for name := range missingNames {
			names = append(names, name)
		}
		sort.Strings(names)

		synonyms, err := GenerateSynonymsForNamesWithContext(ctx, names, m.activeAPIKey(), m.batchSize, m.model, m.synonymsPerName)
		if err != nil {
			result.SynonymError = err
		} else {
			for name, values := range synonyms {
				m.cache[name] = sanitizeSynonyms(values, m.synonymsPerName)
			}
			AssignSynonymsToTree(m.index.Tree, m.cache, m.synonymsPerName)
			result.Changed = true
		}
	}

	if result.Changed {
		m.dirty = true
		m.index.GeneratedAt = time.Now().UTC()
	}

	result.Stats = ComputeStats(m.index.Tree)
	result.Stats.CollectedNames = len(CollectNamesForLLM(m.index.Tree))
	return result, nil
}

func (m *IndexManager) FlushIfDirty() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.dirty || m.index == nil {
		return false, nil
	}
	if err := SaveSynonymCache(m.cachePath, m.cache); err != nil {
		return false, err
	}
	if err := SaveContextIndex(m.outputPath, m.index); err != nil {
		return false, err
	}
	m.dirty = false
	return true, nil
}

func (m *IndexManager) SnapshotStats() IndexStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.index == nil || m.index.Tree == nil {
		return IndexStats{}
	}
	stats := ComputeStats(m.index.Tree)
	stats.CollectedNames = len(CollectNamesForLLM(m.index.Tree))
	return stats
}

func (m *IndexManager) Search(query string, opts SearchOptions) []SearchResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.index == nil || m.index.Tree == nil {
		return nil
	}
	return SearchHintsWithOptions(m.index, query, opts)
}

func (m *IndexManager) RootPath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rootPath
}

func (m *IndexManager) activeAPIKey() string {
	if !m.useLLM {
		return ""
	}
	return m.apiKey
}
