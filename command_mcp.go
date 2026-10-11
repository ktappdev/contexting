package contexting

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

// MCP search tool input.
type searchToolArgs struct {
	Query   string `json:"query" jsonschema:"Search query - keywords, filenames, or concepts"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Max results (default 10)"`
	Type    string `json:"type,omitempty" jsonschema:"Filter: all, files, or dirs"`
	Explain bool   `json:"explain,omitempty" jsonschema:"Include score breakdown"`
	Hybrid  bool   `json:"hybrid,omitempty" jsonschema:"Enable content fallback via ripgrep when index results are sparse"`
}

// MCP status tool has no input.
type statusToolArgs struct{}

// defaultMCPLimit and maxMCPResults bound the search tool's result count so a client
// cannot dump the whole index (or an unbounded response) into an agent's context.
const (
	defaultMCPLimit = 10
	maxMCPResults   = 50
)

// searchToolSchema is the JSON Schema for the search tool's input. It is hand-written
// rather than inferred because the go-sdk's inferred schemas only carry field
// descriptions; enum and maximum constraints require an explicit schema.
var searchToolSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"query": map[string]any{
			"type":        "string",
			"description": "Search query - keywords, filenames, or concepts",
		},
		"limit": map[string]any{
			"type":        "integer",
			"description": "Max results (default 10, maximum 50)",
			"default":     defaultMCPLimit,
			"minimum":     1,
			"maximum":     maxMCPResults,
		},
		"type": map[string]any{
			"type":        "string",
			"description": "Filter: all, files, or dirs",
			"enum":        []any{"all", "files", "dirs"},
		},
		"explain": map[string]any{
			"type":        "boolean",
			"description": "Include score breakdown",
		},
		"hybrid": map[string]any{
			"type":        "boolean",
			"description": "Enable content fallback via ripgrep when index results are sparse",
		},
	},
	"required": []any{"query"},
}

// registerMCPSearchTool exposes concept-based ranked file search over MCP. Search is
// strictly read-only (the manager keeps the index in memory), so it is annotated
// read-onlyHint to let clients auto-approve it.
func registerMCPSearchTool(server *mcp.Server, manager *IndexManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "search",
		Description: "Search a codebase for files using concept-based ranked search. Faster and more relevant than grep or find for locating WHERE code lives. " +
			"Query with plain keywords, concepts, partial filenames, or symbol names (e.g. \"auth login\", \"jwt token refresh\", \"payment handler\", \"createUser\"). " +
			"Results are ranked by relevance score (exact basename +15 and exact symbol +24 rank highest; partial path and basename matches add +4..+7; synonym matches add +5..+8). " +
			"The live index updates automatically as files change, so no separate watch or init step is required while this server is running. " +
			"Set hybrid=true to enable content fallback via ripgrep when index results are sparse. " +
			"Use this instead of grep when you need to find which file handles a concept — grep finds what's INSIDE files, this finds WHICH files matter. " +
			"Do not use for searching file contents (use grep) or file metadata like size/date/permissions (use find).",
		InputSchema: searchToolSchema,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, searchToolHandler(manager))
}

func searchToolHandler(manager *IndexManager) func(ctx context.Context, req *mcp.CallToolRequest, args searchToolArgs) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args searchToolArgs) (*mcp.CallToolResult, any, error) {
		if args.Query == "" {
			return mcpErrorResult("missing required argument: query"), nil, nil
		}
		limit := args.Limit
		if limit <= 0 {
			limit = defaultMCPLimit
		}
		if limit > maxMCPResults {
			limit = maxMCPResults
		}
		typeFilter := args.Type
		if typeFilter == "" {
			typeFilter = "all"
		}
		switch typeFilter {
		case "all", "files", "dirs":
		default:
			return mcpErrorResult(fmt.Sprintf("invalid type %q; must be one of all, files, or dirs", typeFilter)), nil, nil
		}
		results := manager.Search(args.Query, SearchOptions{
			Limit:           limit,
			MinScore:        1,
			TypeFilter:      typeFilter,
			IncludeDebug:    args.Explain,
			ContentFallback: args.Hybrid,
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcpTextResult(formatMCPSearchResults(results, args.Explain))},
		}, nil, nil
	}
}

func registerMCPStatusTool(server *mcp.Server, manager *IndexManager) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "status",
		Description: "Report the health and coverage of the ctxt codebase index. " +
			"Returns total indexed files, directories, index generation time, and root path. " +
			"The server keeps an in-memory index current as files change; use this to check what the live index covers before searching. " +
			"This tool is read-only and never modifies project state.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ statusToolArgs) (*mcp.CallToolResult, any, error) {
		stats := manager.SnapshotStats()
		root := manager.RootPath()
		generatedAt := manager.IndexGeneratedAt()
		var sb strings.Builder
		fmt.Fprintf(&sb, "Root: %s\n", root)
		fmt.Fprintf(&sb, "Total nodes: %d\n", stats.TotalNodes)
		fmt.Fprintf(&sb, "Files: %d\n", stats.TotalFiles)
		fmt.Fprintf(&sb, "Directories: %d\n", stats.TotalDirs)
		fmt.Fprintf(&sb, "Generated: %s\n", generatedAt.Format(time.RFC3339))
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcpTextResult(sb.String())},
		}, nil, nil
	})
}

func mcpErrorResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{mcpTextResult(text)},
	}
}

func mcpTextResult(text string) *mcp.TextContent {
	return &mcp.TextContent{Text: text}
}

func formatMCPSearchResults(results []SearchResult, explain bool) string {
	var sb strings.Builder
	if len(results) == 0 {
		sb.WriteString("No results found.")
		return sb.String()
	}
	for i, r := range results {
		fmt.Fprintf(&sb, "%d. %s (score: %d, type: %s)", i+1, r.Path, r.Score, r.Type)
		if len(r.Matches) > 0 {
			fmt.Fprintf(&sb, " matches: %s", strings.Join(r.Matches, ", "))
		}
		sb.WriteString("\n")
		if explain && len(r.Breakdown) > 0 {
			for _, b := range r.Breakdown {
				fmt.Fprintf(&sb, "   - %s\n", b)
			}
		}
	}
	return sb.String()
}

func newMCPCommand() *cobra.Command {
	flags := CommonFlags{}
	var debounce time.Duration
	var llmOnWatch bool
	var persist string
	var persistInterval time.Duration
	var searchLog bool
	var searchLogQueryMax int
	var maxBatchSize int
	var enableHTTP bool

	cmd := &cobra.Command{
		Use:   "mcp [path]",
		Short: "Start MCP server — watch directory and expose search+status tools over stdio for AI assistants",
		Long: `Starts a Model Context Protocol (MCP) server that AI assistants (Claude Desktop, Cursor, etc.) can use to search your codebase.

Exposes two tools:
  search - Concept-based ranked file search using the precomputed index (symbols, synonyms, paths).
  status - Index health check (file count, generation time, root path).

The server watches for file changes and keeps the index current. All communication is via stdin/stdout JSON-RPC — no network, fully local.

Pass the project's absolute path so the server targets the right project no matter what working directory the AI client launches it from. If several ctxt servers run on one project, the first becomes the writer and later ones serve read-only (live in-memory search, no snapshot writes).

Setup: Add to your AI client's MCP config:
  {"command": "ctxt", "args": ["mcp", "/abs/path/to/project"]}`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// MCP uses stdout for JSON-RPC; all logging must go to stderr.
			logToStderr = true

			rootArg := ""
			if len(args) == 1 {
				rootArg = args[0]
			}
			absConfigPath, err := resolveStartupConfigPath(configPath, rootArg, cmd.Flags().Changed("config"))
			if err != nil {
				return err
			}
			if absConfigPath != "" && !cmd.Flags().Changed("config") {
				if _, statErr := os.Stat(absConfigPath); os.IsNotExist(statErr) {
					LogWarnf("Config file not found at %s; using defaults.", absConfigPath)
				}
			}
			cfg, err := LoadContextingConfig(absConfigPath)
			if err != nil {
				return err
			}
			applyCommonConfig(cmd, &flags, cfg.Common)
			if flags.SymbolExtractor != "" {
				SymbolsExtractorMode = flags.SymbolExtractor
			}
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
				return fmt.Errorf("persistence mode %q is unsupported; use --persist=shutdown", persistMode)
			}
			if persistInterval <= 0 {
				persistInterval = 45 * time.Second
			}

			rootPath := "."
			if rootArg != "" {
				rootPath = rootArg
			} else if cfg.Watch.RootPath != "" {
				rootPath = cfg.Watch.RootPath
			}

			absRoot, err := filepath.Abs(rootPath)
			if err != nil {
				return fmt.Errorf("resolve mcp path: %w", err)
			}
			releaseWriter, writeMode, err := acquireWriterGuardAllowShared(absRoot)
			if err != nil {
				return err
			}
			if writeMode {
				defer releaseWriter()
			}
			outputPath := resolveProjectPath(absRoot, flags.OutputPath)
			if _, statErr := os.Stat(outputPath); os.IsNotExist(statErr) {
				return fmt.Errorf("no index found at %s; run 'ctxt init' from the project root first", outputPath)
			}
			cachePath := resolveProjectPath(absRoot, flags.SynonymCache)
			runtimeFile := resolveProjectPath(absRoot, ".ctxt/ctx_runtime.json")

			ignored, err := BuildIgnoreMapForRoot(absRoot, flags.ExtraIgnores)
			if err != nil {
				return err
			}
			EmbedDotWhitelist(ignored, BuildDotWhitelist(cfg.Common.DotWhitelist))
			if isInsideProject(absConfigPath, absRoot) {
				ignored[filepath.Base(absConfigPath)] = true
				ignored[filepath.Base(absConfigPath)+".example"] = true
			}
			if isInsideProject(outputPath, absRoot) {
				ignored[filepath.Base(outputPath)] = true
			}

			llmEndpoint, llmModel, llmKey, llmTemp, llmMaxTokens, llmProvider := resolveLLMConfig(flags, cfg.LLM)
			LogInfof("LLM: provider=%s model=%s endpoint=%s api_key=%s", llmProvider, llmModel, endpointForLog(llmEndpoint), maskAPIKey(llmKey))
			if !llmOnWatch {
				llmKey = ""
				LogInfof("MCP LLM mode is off. Using cache + lexical synonyms only.")
			}
			if llmOnWatch && llmKey == "" {
				LogWarnf("LLM API key not configured; continuing without synonyms")
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
				SynonymsMin:     flags.SynonymsMin,
				SynonymsMax:     flags.SynonymsMax,
				APIKey:          llmKey,
				UseLLM:          llmOnWatch && llmKey != "",
				MaxBatchSize:    maxBatchSize,
				Endpoint:        llmEndpoint,
				Temperature:     llmTemp,
				MaxTokens:       llmMaxTokens,
			})

			bootstrapStats, err := manager.Bootstrap(ctx)
			if err != nil {
				if isCanceledError(err) {
					LogInfof("Startup indexing canceled.")
					return nil
				}
				return err
			}
			LogInfof("In-memory index ready: %d nodes (%d files, %d directories).", bootstrapStats.TotalNodes, bootstrapStats.TotalFiles, bootstrapStats.TotalDirs)

			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return fmt.Errorf("create watcher: %w", err)
			}
			defer watcher.Close()

			watchedDirs := make(map[string]struct{})
			if err := syncWatchDirectories(watcher, absRoot, ignored, watchedDirs); err != nil {
				return err
			}

			LogInfof("Watching %s for changes...", absRoot)
			LogInfof("MCP settings: debounce=%s verbose=%t persist=%s output=%s cache=%s http=%t", debounce.String(), flags.Verbose, persistMode, outputPath, cachePath, enableHTTP)

			var memoryServer *memorySearchServer
			if enableHTTP {
				if searchLogQueryMax <= 0 {
					searchLogQueryMax = defaultSearchLogQueryMax
				}
				var err error
				memoryServer, err = startMemorySearchServer(ctx, manager, runtimeFile, MemorySearchLogOptions{
					Enabled:  searchLog,
					QueryMax: searchLogQueryMax,
				})
				if err != nil {
					return err
				}
				defer func() {
					_ = memoryServer.Close()
				}()
				LogInfof("Memory search endpoint ready at %s", memoryServer.Address())
			}

			var persistTicker *time.Ticker
			if persistMode == PersistInterval {
				persistTicker = time.NewTicker(persistInterval)
				defer persistTicker.Stop()
				LogInfof("Periodic flush enabled: interval=%s", persistInterval.String())
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
								LogErrorf("Apply changes failed: %v", applyErr)
							}
							continue
						}
						emitSynonymWarning(result.SynonymError)
						if result.Changed {
							if flags.Verbose {
								LogInfof("In-memory index updated: %d nodes (%d files, %d directories).", result.Stats.TotalNodes, result.Stats.TotalFiles, result.Stats.TotalDirs)
							}
							if persistMode == PersistChange {
								flushed, flushErr := manager.FlushIfDirty()
								if flushErr != nil {
									LogErrorf("Change-triggered flush failed: %v", flushErr)
								} else if flushed && flags.Verbose {
									LogInfof("Saved snapshot after change to %s", outputPath)
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

			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case err := <-watcher.Errors:
						if err != nil {
							LogErrorf("Watcher error: %v", err)
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
							LogInfof("Event: %s %s", event.Op, event.Name)
						}

						if event.Op&(fsnotify.Create|fsnotify.Rename) != 0 {
							if err := syncWatchDirectories(watcher, absRoot, ignored, watchedDirs); err != nil {
								LogErrorf("Sync watch dirs failed: %v", err)
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
							LogErrorf("Periodic flush failed: %v", flushErr)
							continue
						}
						if flushed {
							LogInfof("Periodic flush wrote snapshot to %s", outputPath)
						}
					case <-timer.C:
						if !dirty {
							continue
						}
						dirty = false
						enqueueApply()
					}
				}
			}()

			server := mcp.NewServer(&mcp.Implementation{
				Name:    "ctxt",
				Version: currentVersion(),
			}, nil)

			registerMCPSearchTool(server, manager)
			registerMCPStatusTool(server, manager)

			LogInfof("MCP server ready on stdio.")
			serverErr := server.Run(ctx, &mcp.StdioTransport{})

			remaining := drainPending()
			if len(remaining) > 0 {
				logChangeSummary(remaining, flags.Verbose)
				result, applyErr := manager.ApplyChanges(context.Background(), remaining)
				if applyErr != nil {
					LogErrorf("Final apply failed: %v", applyErr)
				} else {
					emitSynonymWarning(result.SynonymError)
				}
			}
			if writeMode {
				flushed, flushErr := manager.FlushIfDirty()
				if flushErr != nil {
					LogErrorf("Failed to flush snapshot on shutdown: %v", flushErr)
				}
				if flushed {
					LogInfof("Flushed snapshot to %s and %s", outputPath, cachePath)
				}
			} else {
				LogInfof("Read-only MCP server: skipping snapshot flush.")
			}
			LogInfof("Stopping MCP server.")
			if serverErr != nil {
				return fmt.Errorf("mcp server: %w", serverErr)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&flags.OutputPath, "output", "o", ".ctxt/ctx_index.json", "Output JSON path")
	cmd.Flags().StringVar(&flags.Model, "llm-model", "", "LLM model used for synonym generation")
	cmd.Flags().StringVar(&flags.APIKey, "api-key", "", "LLM API key (falls back to config api_key_env, LLM_API_KEY, OPENROUTER_API_KEY)")
	cmd.Flags().StringVar(&flags.Endpoint, "llm-endpoint", "", "LLM API endpoint URL")
	cmd.Flags().IntVar(&flags.BatchSize, "batch-size", 0, "Names per LLM request (0 = automatic, up to 60)")
	cmd.Flags().IntVar(&maxBatchSize, "max-batch-size", 0, "Maximum names per LLM request (0 = send all at once, default)")
	cmd.Flags().IntVar(&flags.SynonymsPerName, "synonyms", defaultSynonyms, "Desired synonyms per name")
	cmd.Flags().IntVar(&flags.SynonymsMin, "synonyms-min", 0, "Min synonyms per name (0 = use synonyms value)")
	cmd.Flags().IntVar(&flags.SynonymsMax, "synonyms-max", 0, "Max synonyms per name (0 = use synonyms value)")
	cmd.Flags().StringVar(&flags.SynonymCache, "synonym-cache", ".ctxt/ctx_cache.json", "Path to persistent synonym cache JSON")
	cmd.Flags().StringSliceVar(&flags.ExtraIgnores, "ignore", nil, "Additional ignore entries (name or relative path)")
	cmd.Flags().StringVar(&flags.SymbolExtractor, "symbol-extractor", "auto", "Symbol extraction engine: auto, treesitter, regex")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Enable verbose logging")
	cmd.Flags().DurationVar(&debounce, "debounce", defaultDebounce, "Debounce interval for coalescing fs events")
	cmd.Flags().BoolVar(&llmOnWatch, "llm-on-watch", false, "Enable live LLM synonym generation during MCP watch")
	cmd.Flags().StringVar(&persist, "persist", string(PersistShutdown), "Persistence mode (only shutdown is supported)")
	cmd.Flags().DurationVar(&persistInterval, "persist-interval", defaultPersistInterval, "Reserved for compatibility; unused with shutdown persistence")
	_ = cmd.Flags().MarkHidden("persist-interval")
	cmd.Flags().BoolVar(&searchLog, "search-log", false, "Log incoming queries (may contain sensitive data)")
	cmd.Flags().IntVar(&searchLogQueryMax, "search-log-query-max", defaultSearchLogQueryMax, "Maximum query characters shown in search logs")
	cmd.Flags().BoolVar(&enableHTTP, "http", false, "Also serve HTTP memory search endpoint alongside MCP")

	return cmd
}
