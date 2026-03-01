package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

func newWatchCommand() *cobra.Command {
	flags := CommonFlags{}
	var debounce time.Duration
	var llmOnWatch bool

	cmd := &cobra.Command{
		Use:   "watch [path]",
		Short: "Watch a directory and keep context JSON updated",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadContextingConfig(configPath)
			if err != nil {
				return err
			}
			applyCommonConfig(cmd, &flags, cfg.Common)
			if cfg.Watch.UseLLM != nil && !cmd.Flags().Changed("llm-on-watch") {
				llmOnWatch = *cfg.Watch.UseLLM
			}
			if d, err := cfg.Watch.DebounceDuration(); err != nil {
				return err
			} else if d > 0 && !cmd.Flags().Changed("debounce") {
				debounce = d
			}
			flags.normalize()
			rootPath := "."
			if len(args) == 1 {
				rootPath = args[0]
			} else if cfg.Watch.RootPath != "" {
				rootPath = cfg.Watch.RootPath
			}

			absRoot, err := filepath.Abs(rootPath)
			if err != nil {
				return fmt.Errorf("resolve watch path: %w", err)
			}
			outputPath := resolveProjectPath(absRoot, flags.OutputPath)
			cachePath := resolveProjectPath(absRoot, flags.SynonymCache)

			ignored, err := BuildIgnoreMapForRoot(absRoot, flags.ExtraIgnores)
			if err != nil {
				return err
			}
			apiKey := resolveAPIKey(flags.APIKey)
			if !llmOnWatch {
				apiKey = ""
				logInfof("Watch LLM mode is off (default). Using cache + lexical synonyms only.")
			}
			cache, err := LoadSynonymCache(cachePath)
			if err != nil {
				return err
			}
			if llmOnWatch && apiKey == "" {
				logWarnf("OPENROUTER_API_KEY not set and --api-key not provided; continuing without synonyms")
			}

			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return fmt.Errorf("create watcher: %w", err)
			}
			defer watcher.Close()

			watchedDirs := make(map[string]struct{})
			if err := syncWatchDirectories(watcher, absRoot, ignored, watchedDirs); err != nil {
				return err
			}

			ctx, stop := signalAwareContext()
			defer stop()

			runIndex := func(reason string) {
				if ctx.Err() != nil {
					return
				}
				if flags.Verbose {
					logInfof("Reindexing (%s)...", reason)
				}
				result, indexErr := BuildIndex(BuildOptions{
					Ctx:             ctx,
					RootPath:        absRoot,
					IgnoredPaths:    ignored,
					APIKey:          apiKey,
					Model:           flags.Model,
					BatchSize:       flags.BatchSize,
					SynonymsPerName: flags.SynonymsPerName,
					SynonymCache:    cache,
				})
				if indexErr != nil {
					if ctx.Err() != nil {
						logInfof("Reindex canceled.")
						return
					}
					logErrorf("Indexing failed: %v", indexErr)
					return
				}
				if ctx.Err() != nil {
					logInfof("Reindex canceled.")
					return
				}
				if isCanceledError(result.SynonymError) {
					logInfof("Reindex canceled.")
					return
				}
				cache = result.SynonymCache
				emitSynonymWarning(result.SynonymError)
				if err := SaveSynonymCache(cachePath, cache); err != nil {
					logErrorf("Failed to write synonym cache %s: %v", cachePath, err)
					return
				}
				if err := SaveContextIndex(outputPath, result.Index); err != nil {
					logErrorf("Failed to write %s: %v", outputPath, err)
					return
				}
				logInfof("Updated %s: %d nodes (%d files, %d directories).", outputPath, result.Stats.TotalNodes, result.Stats.TotalFiles, result.Stats.TotalDirs)
			}

			reindexCh := make(chan string, 1)
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case reason := <-reindexCh:
						runIndex(reason)
					}
				}
			}()
			enqueueReindex(reindexCh, "initial")

			logInfof("Watching %s for changes...", absRoot)
			logInfof("Watch settings: debounce=%s verbose=%t output=%s cache=%s", debounce.String(), flags.Verbose, outputPath, cachePath)

			dirty := false
			pendingChanges := make(map[string]fsnotify.Op)
			timer := time.NewTimer(debounce)
			if !timer.Stop() {
				<-timer.C
			}

			for {
				select {
				case <-ctx.Done():
					logInfof("Stopping watcher.")
					return nil
				case err := <-watcher.Errors:
					if err != nil {
						logErrorf("Watcher error: %v", err)
					}
				case event, ok := <-watcher.Events:
					if !ok {
						return nil
					}
					if shouldSkipEvent(absRoot, event, ignored, outputPath, cachePath) {
						continue
					}
					relName := event.Name
					if rel, relErr := filepath.Rel(absRoot, event.Name); relErr == nil {
						relName = rel
					}
					pendingChanges[relName] = pendingChanges[relName] | event.Op
					if flags.Verbose {
						logInfof("Event: %s %s", event.Op, event.Name)
					}

					if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
						if err := syncWatchDirectories(watcher, absRoot, ignored, watchedDirs); err != nil {
							logErrorf("Sync watch dirs failed: %v", err)
						}
					}

					dirty = true
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(debounce)
				case <-timer.C:
					if !dirty {
						continue
					}
					dirty = false
					logChangeSummary(pendingChanges)
					pendingChanges = make(map[string]fsnotify.Op)
					enqueueReindex(reindexCh, "file change")
					if err := syncWatchDirectories(watcher, absRoot, ignored, watchedDirs); err != nil {
						logErrorf("Sync watch dirs failed: %v", err)
					}
				}
			}
		},
	}

	cmd.Flags().StringVarP(&flags.OutputPath, "output", "o", "context.json", "Output JSON path")
	cmd.Flags().StringVar(&flags.Model, "llm-model", defaultModel, "LLM model used for synonym generation")
	cmd.Flags().StringVar(&flags.APIKey, "api-key", "", "OpenRouter API key (falls back to OPENROUTER_API_KEY)")
	cmd.Flags().IntVar(&flags.BatchSize, "batch-size", 8, "Names per LLM request")
	cmd.Flags().IntVar(&flags.SynonymsPerName, "synonyms", 4, "Desired synonyms per name")
	cmd.Flags().StringVar(&flags.SynonymCache, "synonym-cache", ".contexting_synonyms_cache.json", "Path to persistent synonym cache JSON")
	cmd.Flags().StringSliceVar(&flags.ExtraIgnores, "ignore", nil, "Additional ignore entries (name or relative path)")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", true, "Enable verbose logging")
	cmd.Flags().DurationVar(&debounce, "debounce", 750*time.Millisecond, "Debounce interval for reindexing")
	cmd.Flags().BoolVar(&llmOnWatch, "llm-on-watch", false, "Enable live LLM synonym generation during watch (off by default for responsiveness)")

	return cmd
}

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

func enqueueReindex(ch chan string, reason string) {
	select {
	case ch <- reason:
	default:
		// A reindex is already queued/running; coalesce bursts into existing work.
	}
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

	logInfof("Detected changes: created=%d modified=%d removed=%d renamed=%d", created, modified, removed, renamed)
	const maxDetails = 10
	if len(details) <= maxDetails {
		logInfof("Changed paths: %s", strings.Join(details, ", "))
		return
	}
	logInfof("Changed paths: %s, ... and %d more", strings.Join(details[:maxDetails], ", "), len(details)-maxDetails)
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
