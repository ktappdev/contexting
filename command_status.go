package contexting

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

type StatusReport struct {
	ProjectRoot      string     `json:"project_root"`
	ConfigPath       string     `json:"config_path"`
	ConfigExists     bool       `json:"config_exists"`
	IndexPath        string     `json:"index_path"`
	CachePath        string     `json:"cache_path"`
	RuntimePath      string     `json:"runtime_path"`
	IndexExists      bool       `json:"index_exists"`
	IndexGeneratedAt *time.Time `json:"index_generated_at,omitempty"`
	CacheExists      bool       `json:"cache_exists"`
	CacheEntries     int        `json:"cache_entries,omitempty"`
	WatchRunning     bool       `json:"watch_running"`
	WatchPID         int        `json:"watch_pid,omitempty"`
	WatchAddress     string     `json:"watch_address,omitempty"`
	WatchStartedAt   *time.Time `json:"watch_started_at,omitempty"`
}

func newStatusCommand() *cobra.Command {
	var rootPath string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show project status: paths, index health, watch state",
		Long: `Shows current project status — index file path, file count, last generation time, watch state, and synonym cache info.

Use this to verify that 'ctxt init' or 'ctxt watch' is working correctly.

Examples:
  ctxt status                     Pretty-printed status (default)
  ctxt status --root /path/to/project   Check a different project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var absConfigPath string
			if configPath != "" {
				var err error
				absConfigPath, err = filepath.Abs(configPath)
				if err != nil {
					return fmt.Errorf("resolve config path: %w", err)
				}
			}

			cfg, _ := LoadContextingConfig(absConfigPath)
			common := defaultCommonFlags()
			applyCommonConfigNoCLI(&common, cfg.Common)
			common.normalize()

			if rootPath == "" {
				if cfg.Init.RootPath != "" {
					rootPath = cfg.Init.RootPath
				} else if cfg.Watch.RootPath != "" {
					rootPath = cfg.Watch.RootPath
				} else {
					rootPath = "."
				}
			}
			absRoot, err := filepath.Abs(rootPath)
			if err != nil {
				return fmt.Errorf("resolve root path: %w", err)
			}

			var indexPath string
			if cfg.Search.IndexPath != "" {
				indexPath = cfg.Search.IndexPath
			} else {
				indexPath = common.OutputPath
			}
			indexPath = resolveConfigPath(absConfigPath, indexPath)
			if indexPath == "" {
				indexPath = resolveProjectPath(absRoot, ".ctxt/ctx_index.json")
			}

			cachePath := resolveProjectPath(absRoot, common.SynonymCache)
			runtimeFile := resolveProjectPath(absRoot, ".ctxt/ctx_runtime.json")

			report := StatusReport{
				ProjectRoot:  absRoot,
				ConfigPath:   absConfigPath,
				IndexPath:    indexPath,
				CachePath:    cachePath,
				RuntimePath:  runtimeFile,
			}

			if absConfigPath == "" {
				report.ConfigPath = resolveProjectPath(absRoot, ".ctxt/ctx_config.toml")
			}
			if _, err := os.Stat(report.ConfigPath); err == nil {
				report.ConfigExists = true
			}

			if _, err := os.Stat(indexPath); err == nil {
				report.IndexExists = true
				if index, loadErr := LoadContextIndex(indexPath); loadErr == nil && !index.GeneratedAt.IsZero() {
					report.IndexGeneratedAt = &index.GeneratedAt
				}
			}

			if _, err := os.Stat(cachePath); err == nil {
				report.CacheExists = true
				if cache, loadErr := LoadSynonymCache(cachePath); loadErr == nil {
					report.CacheEntries = len(cache)
				}
			}

			if _, err := os.Stat(runtimeFile); err == nil {
				if state, loadErr := LoadRuntimeState(runtimeFile); loadErr == nil {
					report.WatchPID = state.PID
					report.WatchAddress = state.Address
					if !state.StartedAt.IsZero() {
						report.WatchStartedAt = &state.StartedAt
					}
					if isProcessAlive(state.PID) {
						report.WatchRunning = true
					}
				}
			}

			if jsonOut {
				bytes, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(bytes))
				return nil
			}

			printStatusReport(report)
			return nil
		},
	}

	cmd.Flags().StringVar(&rootPath, "root", "", "Project root path (defaults to current working directory)")
	cmd.Flags().BoolVar(&jsonOut, "json", true, "Output status as JSON")

	return cmd
}

func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(os.Signal(nil)) == nil
}

func printStatusReport(report StatusReport) {
	status := func(label, value string) {
		fmt.Printf("%-18s %s\n", label+":", value)
	}
	boolStr := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}

	status("Project root", report.ProjectRoot)
	status("Config", report.ConfigPath)
	status("Config exists", boolStr(report.ConfigExists))
	status("Index", report.IndexPath)
	status("Index exists", boolStr(report.IndexExists))
	if report.IndexGeneratedAt != nil {
		status("Index generated", report.IndexGeneratedAt.Format(time.RFC3339))
	}
	status("Cache", report.CachePath)
	status("Cache exists", boolStr(report.CacheExists))
	if report.CacheExists {
		status("Cache entries", fmt.Sprintf("%d", report.CacheEntries))
	}
	status("Runtime", report.RuntimePath)
	status("Watch running", boolStr(report.WatchRunning))
	if report.WatchRunning {
		status("Watch PID", fmt.Sprintf("%d", report.WatchPID))
		status("Watch address", report.WatchAddress)
		if report.WatchStartedAt != nil {
			status("Watch started", report.WatchStartedAt.Format(time.RFC3339))
		}
	}
}
