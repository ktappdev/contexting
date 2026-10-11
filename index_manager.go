package contexting

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
	DotWhitelist    []string // Extra dot files merged with defaults
	Model           string
	BatchSize       int
	SynonymsPerName int
	SynonymsMax     int
	APIKey          string
	UseLLM          bool
	MaxBatchSize    int
	Endpoint        string
	Temperature     float64
	MaxTokens       int
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
	synonymsMax     int
	apiKey          string
	useLLM          bool
	maxBatchSize    int
	endpoint        string
	temperature     float64
	maxTokens       int

	index        *ContextIndex
	cache        SynonymResponse
	fingerprints map[string]string
	dirty        bool
	loaded       bool
}

func NewIndexManager(opts IndexManagerOptions) *IndexManager {
	cache := make(SynonymResponse)
	if opts.Model == "" {
		opts.Model = defaultModel
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 0 // default: send all
	}
	if opts.SynonymsPerName <= 0 {
		opts.SynonymsPerName = defaultSynonyms
	}
	if opts.SynonymsMax <= 0 {
		opts.SynonymsMax = defaultSynonymsMax
	}
	if opts.IgnoredPaths == nil {
		opts.IgnoredPaths = BuildIgnoreMap(nil)
	}
	if len(opts.DotWhitelist) > 0 {
		EmbedDotWhitelist(opts.IgnoredPaths, BuildDotWhitelist(opts.DotWhitelist))
	}

	return &IndexManager{
		rootPath:        opts.RootPath,
		outputPath:      opts.OutputPath,
		cachePath:       opts.CachePath,
		ignored:         opts.IgnoredPaths,
		model:           opts.Model,
		batchSize:       opts.BatchSize,
		synonymsPerName: opts.SynonymsPerName,
		synonymsMax:     opts.SynonymsMax,
		apiKey:          opts.APIKey,
		useLLM:          opts.UseLLM,
		maxBatchSize:    opts.MaxBatchSize,
		endpoint:        opts.Endpoint,
		temperature:     opts.Temperature,
		maxTokens:       opts.MaxTokens,
		cache:           cache,
	}
}

type fsEntry struct {
	mtime int64
	size  int64
	isDir bool
}

func collectFilesystemState(rootPath string, ignored map[string]bool) (map[string]fsEntry, error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}

	state := make(map[string]fsEntry)
	matcher := newIgnoreMatcher(ignored)
	fileCount := 0
	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		if matcher.shouldIgnore(rel, d.Name(), d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		normalized := filepath.ToSlash(rel)
		if d.IsDir() {
			state[normalized] = fsEntry{isDir: true}
		} else {
			fileCount++
			if fileCount > MaxFileCount {
				return fmt.Errorf("project exceeds %d files; add ignore patterns", MaxFileCount)
			}
			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("stat %s: %w", path, err)
			}
			state[normalized] = fsEntry{mtime: info.ModTime().UnixNano(), size: info.Size()}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}

func collectSnapshotPaths(tree *Node) map[string]*Node {
	paths := make(map[string]*Node)
	if tree == nil {
		return paths
	}
	walkTree(tree, func(node *Node) {
		if node == tree {
			return
		}
		rel, err := filepath.Rel(tree.FullPath, node.FullPath)
		if err != nil {
			return
		}
		paths[filepath.ToSlash(rel)] = node
	})
	return paths
}

func (m *IndexManager) Bootstrap(ctx context.Context) (IndexStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := loadSynonymCacheState(m.cachePath)
	if err != nil {
		return IndexStats{}, err
	}
	m.cache = state.Entries
	m.fingerprints = state.Fingerprints

	loadedIndex, err := LoadContextIndex(m.outputPath)
	if err == nil && loadedIndex != nil && loadedIndex.Tree != nil {
		if absRoot, rootErr := filepath.Abs(m.rootPath); rootErr == nil && loadedIndex.RootPath == absRoot {
			m.index = loadedIndex
			m.loaded = true
			m.dirty = false

			// Diff snapshot against filesystem to catch changes made while not running
			fsState, fsErr := collectFilesystemState(m.rootPath, m.ignored)
			if fsErr != nil {
				LogWarnf("Unable to scan filesystem for staleness check: %v — falling back to full rebuild", fsErr)
			} else {
				snapPaths := collectSnapshotPaths(m.index.Tree)
				changesDetected := false
				var newCount, deletedCount, modifiedCount int
				newNamesNeedSynonyms := make(map[string]struct{})

				// Remove deleted files/dirs from snapshot
				for relPath := range snapPaths {
					if _, exists := fsState[relPath]; !exists {
						if removeNodeByRelPath(m.index.Tree, relPath) {
							changesDetected = true
							deletedCount++
						}
					}
				}

				// Add new or update modified files/dirs
				for relPath, entry := range fsState {
					node, exists := snapPaths[relPath]
					if !exists {
						// New file/dir not in snapshot
						if upsertNodeByRelPath(m.index.Tree, absRoot, relPath, entry.isDir, entry.mtime, m.cache, m.synonymsMax) {
							changesDetected = true
							newCount++
							if !entry.isDir {
								name := filepath.ToSlash(relPath)
								if _, cached := m.cache[name]; !cached {
									newNamesNeedSynonyms[name] = struct{}{}
								}
							}
						}
						continue
					}
					// Check type mismatch (dir↔file)
					nodeIsDir := node.Type == "directory"
					if nodeIsDir != entry.isDir {
						if upsertNodeByRelPath(m.index.Tree, absRoot, relPath, entry.isDir, entry.mtime, m.cache, m.synonymsMax) {
							changesDetected = true
							modifiedCount++
						}
						continue
					}
					// Compare size too: tools can preserve timestamps when replacing files.
					if !entry.isDir && (entry.mtime != node.ModTime || entry.size != node.Size) {
						if upsertNodeByRelPath(m.index.Tree, absRoot, relPath, entry.isDir, entry.mtime, m.cache, m.synonymsMax) {
							changesDetected = true
							modifiedCount++
						}
					}
				}

				if changesDetected {
					m.dirty = true
					m.index.GeneratedAt = time.Now().UTC()
					LogWarnf("########################################")
					LogWarnf("# Bootstrap diff: +%d new, -%d deleted, %d modified", newCount, deletedCount, modifiedCount)
					if len(newNamesNeedSynonyms) > 0 {
						LogWarnf("# Run: ctxt sync")
						LogWarnf("# to generate synonyms for %d new names", len(newNamesNeedSynonyms))
					}
					LogWarnf("########################################")
				}
				AssignSynonymsToTree(m.index.Tree, m.cache, m.synonymsMax)
				stats := ComputeStats(m.index.Tree)
				stats.CollectedNames = len(CollectNamesForLLM(m.index.Tree))
				return stats, nil
			}
			m.loaded = false
			m.index = nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		LogWarnf("Unable to load existing context snapshot: %v", err)
	}

	result, buildErr := BuildIndex(BuildOptions{
		Ctx:                 ctx,
		RootPath:            m.rootPath,
		IgnoredPaths:        m.ignored,
		APIKey:              m.activeAPIKey(),
		Model:               m.model,
		BatchSize:           m.batchSize,
		SynonymsPerName:     m.synonymsMax,
		SynonymsMax:         m.synonymsMax,
		SynonymCache:        m.cache,
		SynonymFingerprints: m.fingerprints,
		MaxBatchSize:        m.maxBatchSize,
		Endpoint:            m.endpoint,
		Temperature:         m.temperature,
		MaxTokens:           m.maxTokens,
	})
	if buildErr != nil {
		return IndexStats{}, buildErr
	}
	m.index = result.Index
	m.cache = result.SynonymCache
	m.fingerprints = result.SynonymFingerprints
	m.dirty = true
	m.loaded = true
	return result.Stats, nil
}

func (m *IndexManager) ApplyChanges(ctx context.Context, changes map[string]fsnotify.Op) (ApplyResult, error) {
	m.mu.Lock()
	locked := true
	defer func() {
		if locked {
			m.mu.Unlock()
		}
	}()

	if !m.loaded || m.index == nil || m.index.Tree == nil {
		return ApplyResult{}, fmt.Errorf("index manager not bootstrapped")
	}

	result := ApplyResult{}
	if len(changes) == 0 {
		result.Stats = ComputeStats(m.index.Tree)
		result.Stats.CollectedNames = len(CollectNamesForLLM(m.index.Tree))
		return result, nil
	}

	// Directory creation can arrive after its children were populated. Reconcile
	// the filesystem on creates so those children are not missed by fsnotify.
	for _, op := range changes {
		if op&fsnotify.Create != 0 {
			state, err := collectFilesystemState(m.rootPath, m.ignored)
			if err != nil {
				return result, err
			}
			merged := make(map[string]fsnotify.Op, len(changes)+len(state))
			for path, event := range changes {
				merged[path] = event
			}
			snapshot := collectSnapshotPaths(m.index.Tree)
			for path, entry := range state {
				if node, ok := snapshot[path]; !ok || (node.Type == "directory") != entry.isDir || (!entry.isDir && (node.ModTime != entry.mtime || node.Size != entry.size)) {
					merged[path] |= fsnotify.Create
				}
			}
			changes = merged
			break
		}
	}
	matcher := newIgnoreMatcher(m.ignored)
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
		if !isInsideProject(absPath, m.rootPath) {
			return result, fmt.Errorf("change path is outside project: %s", relPath)
		}
		baseName := filepath.Base(absPath)

		if op&(fsnotify.Remove|fsnotify.Rename) != 0 {
			if removeNodeByRelPath(m.index.Tree, relPath) {
				result.Changed = true
			}
		}

		if op&(fsnotify.Create|fsnotify.Write|fsnotify.Chmod) == 0 {
			continue
		}

		isDir, mtime, statErr := isExistingDirectory(absPath)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return result, statErr
		}

		if matcher.shouldIgnore(relPath, baseName, isDir) {
			if removeNodeByRelPath(m.index.Tree, relPath) {
				result.Changed = true
			}
			continue
		}

		if upsertNodeByRelPath(m.index.Tree, m.rootPath, relPath, isDir, mtime, m.cache, m.synonymsMax) {
			result.Changed = true
		}
	}

	if ctx.Err() != nil {
		return result, ctx.Err()
	}

	generationContext := synonymContext{}
	var names []string
	if m.useLLM {
		generationContext = collectSynonymContext(m.index.Tree)
		names = namesNeedingSynonyms(CollectNamesForLLM(m.index.Tree), m.cache, m.fingerprints, generationContext.Fingerprints)
	}
	if len(names) > 0 {
		apiKey := m.activeAPIKey()
		m.mu.Unlock()
		locked = false
		synonyms, err := GenerateSynonymsForNamesWithContext(ctx, names, apiKey, m.maxBatchSize, m.model, m.endpoint, m.temperature, m.maxTokens, m.synonymsMax, 1, generationContext.Symbols, generationContext.Imports)
		m.mu.Lock()
		locked = true
		if err != nil {
			result.SynonymError = err
		}
		currentContext := collectSynonymContext(m.index.Tree)
		for name, values := range synonyms {
			if currentContext.Fingerprints[name] != generationContext.Fingerprints[name] {
				continue
			}
			m.cache[name] = sanitizeSynonyms(values, m.synonymsMax)
			m.fingerprints[name] = generationContext.Fingerprints[name]
		}
		if len(synonyms) > 0 {
			AssignSynonymsToTree(m.index.Tree, m.cache, m.synonymsMax)
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
	if err := saveSynonymCacheState(m.cachePath, synonymCacheState{Version: currentCacheVersion, Entries: m.cache, Fingerprints: m.fingerprints}); err != nil {
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
	if m.index == nil || m.index.Tree == nil {
		m.mu.Unlock()
		return nil
	}
	snapshot := *m.index
	snapshot.Tree = cloneSearchTree(m.index.Tree)
	m.mu.Unlock()
	return SearchHintsWithOptions(&snapshot, query, opts)
}

func (m *IndexManager) RootPath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rootPath
}

func (m *IndexManager) IndexGeneratedAt() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.index == nil {
		return time.Time{}
	}
	return m.index.GeneratedAt
}

func (m *IndexManager) activeAPIKey() string {
	if !m.useLLM {
		return ""
	}
	return m.apiKey
}
