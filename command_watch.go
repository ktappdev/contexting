package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

// Timeouts used for watching and persistence
const defaultDebounce = 750 * time.Millisecond // Coalesces multiple fs events into single re-index
const defaultPersistInterval = 45 * time.Second

func newWatchCommand() *cobra.Command {
	flags := CommonFlags{}
	var debounce time.Duration
	var llmOnWatch bool
	var persist string
	var persistInterval time.Duration
	var searchLog bool
	var searchLogQueryMax int
	var maxBatchSize int

	cmd := &cobra.Command{
		Use:   "watch [path]",
		Short: "Watch a directory, keep index in memory, and flush snapshot on shutdown",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var absConfigPath string
			if configPath != "" {
				var err error
				absConfigPath, err = filepath.Abs(configPath)
				if err != nil {
					return fmt.Errorf("resolve config path: %w", err)
				}
			}
			cfg, err := LoadContextingConfig(absConfigPath)
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
			if cfg.Watch.Persist != "" && !cmd.Flags().Changed("persist") {
				persist = cfg.Watch.Persist
			}
			if d, err := cfg.Watch.PersistIntervalDuration(); err != nil {
				return err
			} else if d > 0 && !cmd.Flags().Changed("persist-interval") {
				persistInterval = d
			}
			if cfg.Watch.SearchLog != nil {
				applyBoolFlag(cmd, "search-log", &searchLog, *cfg.Watch.SearchLog)
			}
			applyIntFlag(cmd, "search-log-query-max", &searchLogQueryMax, cfg.Watch.SearchLogQueryMax)
			applyIntFlag(cmd, "max-batch-size", &maxBatchSize, cfg.Watch.MaxBatchSize)

			flags.normalize()
			persistMode, err := parsePersistMode(persist)
			if err != nil {
				return err
			}
			if persistMode != PersistShutdown {
				logWarnf("Persistence mode %q requested, but watch now runs shutdown-only persistence. Using shutdown mode.", persistMode)
				persistMode = PersistShutdown
			}
			if persistInterval <= 0 {
				persistInterval = 45 * time.Second
			}

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
			if _, statErr := os.Stat(absConfigPath); os.IsNotExist(statErr) {
				if !cmd.Flags().Changed("config") {
					return fmt.Errorf("config file not found at %s; run contexting from the project root directory", absConfigPath)
				}
			}
			outputPath := resolveProjectPath(absRoot, flags.OutputPath)
			if _, statErr := os.Stat(outputPath); os.IsNotExist(statErr) {
				return fmt.Errorf("no index found at %s; run 'contexting init' from the project root first", outputPath)
			}
			cachePath := resolveProjectPath(absRoot, flags.SynonymCache)
			runtimeFile := resolveProjectPath(absRoot, ".ctx/ctx_runtime.json")

			ignored, err := BuildIgnoreMapForRoot(absRoot, flags.ExtraIgnores)
			if err != nil {
				return err
			}
			EmbedDotWhitelist(ignored, BuildDotWhitelist(cfg.Common.DotWhitelist))
			// Only skip internal files by basename when the resolved paths are inside the project.
			// If --config or --output points outside the project, skipping by basename could
			// incorrectly exclude legitimate project files.
			if isInsideProject(absConfigPath, absRoot) {
				ignored[filepath.Base(absConfigPath)] = true
				ignored[filepath.Base(absConfigPath)+".example"] = true
			}
			if isInsideProject(outputPath, absRoot) {
				ignored[filepath.Base(outputPath)] = true
			}

			llmEndpoint, llmModel, llmKey, llmTemp, llmMaxTokens, llmProvider := resolveLLMConfig(flags, cfg.LLM)
			logInfof("LLM: provider=%s model=%s endpoint=%s api_key=%s", llmProvider, llmModel, llmEndpoint, maskAPIKey(llmKey))
			if !llmOnWatch {
				llmKey = ""
				logInfof("Watch LLM mode is off (default). Using cache + lexical synonyms only.")
			}
			if llmOnWatch && llmKey == "" {
				logWarnf("LLM API key not configured; continuing without synonyms")
			}

			ctx, stop := signalAwareContext()
			defer stop()

			manager := NewIndexManager(IndexManagerOptions{
				RootPath:        absRoot,
				OutputPath:      outputPath,
				CachePath:       cachePath,
				IgnoredPaths:    ignored,
				Model:           llmModel,
				BatchSize:       flags.BatchSize,
				SynonymsPerName: flags.SynonymsPerName,
				APIKey:          llmKey,
				UseLLM:          llmOnWatch,
				MaxBatchSize:    maxBatchSize,
				Endpoint:        llmEndpoint,
				Temperature:     llmTemp,
				MaxTokens:       llmMaxTokens,
			})

			bootstrapStats, err := manager.Bootstrap(ctx)
			if err != nil {
				if isCanceledError(err) {
					logInfof("Startup indexing canceled.")
					return nil
				}
				return err
			}
			logInfof("In-memory index ready: %d nodes (%d files, %d directories).", bootstrapStats.TotalNodes, bootstrapStats.TotalFiles, bootstrapStats.TotalDirs)

			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return fmt.Errorf("create watcher: %w", err)
			}
			defer watcher.Close()

			watchedDirs := make(map[string]struct{})
			if err := syncWatchDirectories(watcher, absRoot, ignored, watchedDirs); err != nil {
				return err
			}

			logInfof("Watching %s for changes...", absRoot)
			logInfof("Watch settings: debounce=%s verbose=%t persist=%s output=%s cache=%s", debounce.String(), flags.Verbose, persistMode, outputPath, cachePath)

			if searchLogQueryMax <= 0 {
				searchLogQueryMax = defaultSearchLogQueryMax
			}
			memoryServer, err := startMemorySearchServer(ctx, manager, runtimeFile, MemorySearchLogOptions{
				Enabled:  searchLog,
				QueryMax: searchLogQueryMax,
			})
			if err != nil {
				return err
			}
			defer func() {
				_ = memoryServer.Close()
			}()
			logInfof("Memory search endpoint ready at %s", memoryServer.Address())

			var persistTicker *time.Ticker
			if persistMode == PersistInterval {
				persistTicker = time.NewTicker(persistInterval)
				defer persistTicker.Stop()
				logInfof("Periodic flush enabled: interval=%s", persistInterval.String())
			}

			pendingChanges := make(map[string]fsnotify.Op)
			var pendingMu sync.Mutex
			applyTrigger := make(chan struct{}, 1)

			drainPending := func() map[string]fsnotify.Op {
				pendingMu.Lock()
				defer pendingMu.Unlock()
				if len(pendingChanges) == 0 {
					return nil
				}
				copyMap := make(map[string]fsnotify.Op, len(pendingChanges))
				for path, op := range pendingChanges {
					copyMap[path] = op
				}
				pendingChanges = make(map[string]fsnotify.Op)
				return copyMap
			}

			addPending := func(path string, op fsnotify.Op) {
				pendingMu.Lock()
				pendingChanges[path] = pendingChanges[path] | op
				pendingMu.Unlock()
			}

			enqueueApply := func() {
				select {
				case applyTrigger <- struct{}{}:
				default:
				}
			}

			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-applyTrigger:
						changes := drainPending()
						if len(changes) == 0 {
							continue
						}
						logChangeSummary(changes, flags.Verbose)
						result, applyErr := manager.ApplyChanges(ctx, changes)
						if applyErr != nil {
							if !isCanceledError(applyErr) {
								logErrorf("Apply changes failed: %v", applyErr)
							}
							continue
						}
						emitSynonymWarning(result.SynonymError)
						if result.Changed {
							if flags.Verbose {
								logInfof("In-memory index updated: %d nodes (%d files, %d directories).", result.Stats.TotalNodes, result.Stats.TotalFiles, result.Stats.TotalDirs)
							}
							if persistMode == PersistChange {
								flushed, flushErr := manager.FlushIfDirty()
								if flushErr != nil {
									logErrorf("Change-triggered flush failed: %v", flushErr)
								} else if flushed && flags.Verbose {
									logInfof("Saved snapshot after change to %s", outputPath)
								}
							}
						}
					}
				}
			}()

			// Run one startup apply trigger to process any pending setup events quickly.
			enqueueApply()

			dirty := false
			timer := time.NewTimer(debounce)
			if !timer.Stop() {
				<-timer.C
			}

			for {
				select {
				case <-ctx.Done():
					remaining := drainPending()
					if len(remaining) > 0 {
						logChangeSummary(remaining, flags.Verbose)
						result, applyErr := manager.ApplyChanges(context.Background(), remaining)
						if applyErr != nil {
							logErrorf("Final apply failed: %v", applyErr)
						} else {
							emitSynonymWarning(result.SynonymError)
							if result.Changed {
								if flags.Verbose {
									logInfof("In-memory index updated: %d nodes (%d files, %d directories).", result.Stats.TotalNodes, result.Stats.TotalFiles, result.Stats.TotalDirs)
								}
								if persistMode == PersistChange {
									flushed, flushErr := manager.FlushIfDirty()
									if flushErr != nil {
										logErrorf("Final change-triggered flush failed: %v", flushErr)
									} else if flushed && flags.Verbose {
										logInfof("Saved snapshot after final change to %s", outputPath)
									}
								}
							}
						}
					}
					flushed, flushErr := manager.FlushIfDirty()
					if flushErr != nil {
						logErrorf("Failed to flush snapshot on shutdown: %v", flushErr)
						logInfof("Stopping watcher.")
						return flushErr
					}
					if flushed {
						logInfof("Flushed snapshot to %s and %s", outputPath, cachePath)
					} else {
						logInfof("No snapshot flush needed.")
					}
					logInfof("Stopping watcher.")
					return nil
				case err := <-watcher.Errors:
					if err != nil {
						logErrorf("Watcher error: %v", err)
					}
				case event, ok := <-watcher.Events:
					if !ok {
						continue
					}
					if shouldSkipEvent(absRoot, event, ignored, outputPath, cachePath, absConfigPath) {
						continue
					}
					relName := event.Name
					if rel, relErr := filepath.Rel(absRoot, event.Name); relErr == nil {
						relName = rel
					}
					addPending(relName, event.Op)
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
				case <-tickerChan(persistTicker):
					flushed, flushErr := manager.FlushIfDirty()
					if flushErr != nil {
						logErrorf("Periodic flush failed: %v", flushErr)
						continue
					}
					if flushed {
						logInfof("Periodic flush wrote snapshot to %s", outputPath)
					}
				case <-timer.C:
					if !dirty {
						continue
					}
					dirty = false
					enqueueApply()
				}
			}
		},
	}

	cmd.Flags().StringVarP(&flags.OutputPath, "output", "o", ".ctx/ctx_index.json", "Output JSON path")
	cmd.Flags().StringVar(&flags.Model, "llm-model", "", "LLM model used for synonym generation")
	cmd.Flags().StringVar(&flags.APIKey, "api-key", "", "LLM API key (falls back to config api_key_env, LLM_API_KEY, OPENROUTER_API_KEY)")
	cmd.Flags().StringVar(&flags.Endpoint, "llm-endpoint", "", "LLM API endpoint URL")
	cmd.Flags().IntVar(&flags.BatchSize, "batch-size", 0, "Names per LLM request (0 = send all, legacy option)")
	cmd.Flags().IntVar(&maxBatchSize, "max-batch-size", 0, "Maximum names per LLM request (0 = send all at once, default)")
	cmd.Flags().IntVar(&flags.SynonymsPerName, "synonyms", defaultSynonyms, "Desired synonyms per name")
	cmd.Flags().StringVar(&flags.SynonymCache, "synonym-cache", ".ctx/ctx_cache.json", "Path to persistent synonym cache JSON")
	cmd.Flags().StringSliceVar(&flags.ExtraIgnores, "ignore", nil, "Additional ignore entries (name or relative path)")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")
	cmd.Flags().DurationVar(&debounce, "debounce", defaultDebounce, "Debounce interval for coalescing fs events")
	cmd.Flags().BoolVar(&llmOnWatch, "llm-on-watch", true, "Enable live LLM synonym generation during watch (on by default)")
	cmd.Flags().StringVar(&persist, "persist", string(PersistShutdown), "Persistence mode: shutdown|interval|change")
	cmd.Flags().DurationVar(&persistInterval, "persist-interval", defaultPersistInterval, "Snapshot flush interval when --persist=interval")
	cmd.Flags().BoolVar(&searchLog, "search-log", true, "Log incoming memory search queries in watch output")
	cmd.Flags().IntVar(&searchLogQueryMax, "search-log-query-max", defaultSearchLogQueryMax, "Maximum query characters shown in search logs")

	return cmd
}
