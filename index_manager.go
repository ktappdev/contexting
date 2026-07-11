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
	SynonymsMin     int
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
	synonymsMin     int
	synonymsMax     int
	apiKey          string
	useLLM          bool
	maxBatchSize    int
	endpoint        string
	temperature     float64
	maxTokens       int

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
		opts.BatchSize = 0 // default: send all
	}
	if opts.SynonymsPerName <= 0 {
		opts.SynonymsPerName = defaultSynonyms
	}
	if opts.SynonymsMin <= 0 {
		opts.SynonymsMin = opts.SynonymsPerName
	}
	if opts.SynonymsMax <= 0 {
		opts.SynonymsMax = defaultSynonymsMax
	}
	if opts.SynonymsMin > opts.SynonymsMax {
		opts.SynonymsMin = opts.SynonymsMax
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
		synonymsMin:     opts.SynonymsMin,
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
	isDir bool
}

func collectFilesystemState(rootPath string, ignored map[string]bool) (map[string]fsEntry, error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}

	state := make(map[string]fsEntry)
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

		if shouldIgnorePath(rel, d.Name(), ignored) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		normalized := filepath.ToSlash(rel)
		if d.IsDir() {
			state[normalized] = fsEntry{isDir: true}
		} else {
			info, err := d.Info()
			if err != nil {
				return nil // skip files we can't stat
			}
			state[normalized] = fsEntry{mtime: info.ModTime().UnixNano(), isDir: false}
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
								name := filepath.Base(relPath)
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
					// Check modified file (mtime changed)
					if !entry.isDir && entry.mtime > 0 && entry.mtime != node.ModTime {
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
			}

			stats := ComputeStats(m.index.Tree)
			stats.CollectedNames = len(CollectNamesForLLM(m.index.Tree))
			return stats, nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		LogWarnf("Unable to load existing context snapshot: %v", err)
	}

	result, buildErr := BuildIndex(BuildOptions{
		Ctx:             ctx,
		RootPath:        m.rootPath,
		IgnoredPaths:    m.ignored,
		APIKey:          m.activeAPIKey(),
		Model:           m.model,
		BatchSize:       m.batchSize,
		SynonymsPerName: m.synonymsMax,
		SynonymsMin:     m.synonymsMin,
		SynonymsMax:     m.synonymsMax,
		SynonymCache:    m.cache,
		MaxBatchSize:    m.maxBatchSize,
		Endpoint:        m.endpoint,
		Temperature:     m.temperature,
		MaxTokens:       m.maxTokens,
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

		isDir, mtime, statErr := isExistingDirectory(absPath)
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

		if upsertNodeByRelPath(m.index.Tree, m.rootPath, relPath, isDir, mtime, m.cache, m.synonymsMax) {
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

		// Build symbols and imports maps for new names
		symbolsMap := make(map[string][]string)
		importsMap := make(map[string][]string)
		for _, name := range names {
			// Find the node with this basename and extract its symbols
			// and imports. We collect imports from every file with this
			// basename, deduped, so a name like "route.ts" gets the union
			// of all its call sites' dependencies.
			seenImports := make(map[string]struct{})
			walkTree(m.index.Tree, func(node *Node) {
				if node.Type != "file" || filepath.Base(node.FullPath) != name {
					return
				}
				if len(node.Symbols) > 0 {
					symbolsMap[name] = node.Symbols
				}
				// Imports reveal external dependencies (e.g. "@clerk/nextjs")
				// that help the LLM generate domain-accurate synonyms.
				// Imports aren't stored on the node, so we re-extract from disk.
				for _, imp := range extractFileImports(node.FullPath) {
					if _, dup := seenImports[imp]; dup {
						continue
					}
					seenImports[imp] = struct{}{}
				}
			})
			if len(seenImports) > 0 {
				imports := make([]string, 0, len(seenImports))
				for imp := range seenImports {
					imports = append(imports, imp)
				}
				sort.Strings(imports)
				importsMap[name] = imports
			}
		}

		synonyms, err := GenerateSynonymsForNamesWithContext(ctx, names, m.activeAPIKey(), m.maxBatchSize, m.model, m.endpoint, m.temperature, m.maxTokens, m.synonymsMin, m.synonymsMax, 1, symbolsMap, importsMap)
		if err != nil {
			result.SynonymError = err
		} else {
			for name, values := range synonyms {
				m.cache[name] = sanitizeSynonyms(values, m.synonymsMax)
			}
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
