package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type DoctorStatus string

const (
	DoctorPass DoctorStatus = "pass"
	DoctorWarn DoctorStatus = "warn"
	DoctorFail DoctorStatus = "fail"
)

type DoctorCheck struct {
	Name       string       `json:"name"`
	Status     DoctorStatus `json:"status"`
	Message    string       `json:"message"`
	Suggestion string       `json:"suggestion,omitempty"`
}

type DoctorReport struct {
	Healthy bool          `json:"healthy"`
	Checks  []DoctorCheck `json:"checks"`
}

type DoctorOptions struct {
	ConfigPath string
	RootPath   string
	IndexPath  string
	CachePath  string
	WriteCheck bool
}

func RunDoctor(opts DoctorOptions) DoctorReport {
	report := DoctorReport{Healthy: true, Checks: make([]DoctorCheck, 0, 8)}

	cfg := &ContextingConfig{}
	configExists := false
	if opts.ConfigPath != "" {
		if _, err := os.Stat(opts.ConfigPath); err == nil {
			configExists = true
			report.add(DoctorCheck{Name: "config.exists", Status: DoctorPass, Message: "Config file found: " + opts.ConfigPath})
			loaded, err := LoadContextingConfig(opts.ConfigPath)
			if err != nil {
				report.add(DoctorCheck{Name: "config.parse", Status: DoctorFail, Message: err.Error(), Suggestion: "Fix TOML syntax or run `contexting config init --force` to reset."})
			} else {
				cfg = loaded
				report.add(DoctorCheck{Name: "config.parse", Status: DoctorPass, Message: "Config parsed successfully."})
			}
		} else if os.IsNotExist(err) {
			report.add(DoctorCheck{Name: "config.exists", Status: DoctorWarn, Message: "Config file not found: " + opts.ConfigPath, Suggestion: "Run `contexting config init` to create a starter config."})
		} else {
			report.add(DoctorCheck{Name: "config.exists", Status: DoctorFail, Message: err.Error(), Suggestion: "Check file permissions and path."})
		}
	}

	rootPath := opts.RootPath
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
		report.add(DoctorCheck{Name: "root.resolve", Status: DoctorFail, Message: err.Error(), Suggestion: "Use a valid root path."})
		return report
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		report.add(DoctorCheck{Name: "root.exists", Status: DoctorFail, Message: err.Error(), Suggestion: "Create the directory or pass --root to an existing project."})
		return report
	}
	if !info.IsDir() {
		report.add(DoctorCheck{Name: "root.exists", Status: DoctorFail, Message: "Root path is not a directory: " + absRoot, Suggestion: "Pass a directory path with --root."})
		return report
	}
	report.add(DoctorCheck{Name: "root.exists", Status: DoctorPass, Message: "Project root: " + absRoot})

	common := defaultCommonFlags()
	if configExists {
		applyCommonConfigNoCLI(&common, cfg.Common)
	}
	common.normalize()

	indexPath := opts.IndexPath
	if indexPath == "" {
		if cfg.Search.IndexPath != "" {
			indexPath = cfg.Search.IndexPath
		} else {
			indexPath = common.OutputPath
		}
	}
	cachePath := opts.CachePath
	if cachePath == "" {
		cachePath = common.SynonymCache
	}
	indexPath = resolveProjectPath(absRoot, indexPath)
	cachePath = resolveProjectPath(absRoot, cachePath)

	checkIndexFile(&report, indexPath)
	checkCacheFile(&report, cachePath)
	checkAPIKey(&report)
	if opts.WriteCheck {
		checkWriteAccess(&report, absRoot)
	}

	return report
}

func checkIndexFile(report *DoctorReport, indexPath string) {
	if _, err := os.Stat(indexPath); err != nil {
		if os.IsNotExist(err) {
			report.add(DoctorCheck{Name: "index.exists", Status: DoctorWarn, Message: "Index file not found: " + indexPath, Suggestion: "Run `contexting init` to generate context.json."})
			return
		}
		report.add(DoctorCheck{Name: "index.exists", Status: DoctorFail, Message: err.Error(), Suggestion: "Check file permissions and path."})
		return
	}

	index, err := LoadContextIndex(indexPath)
	if err != nil {
		report.add(DoctorCheck{Name: "index.parse", Status: DoctorFail, Message: err.Error(), Suggestion: "Regenerate with `contexting init` if file is corrupted."})
		return
	}
	stats := ComputeStats(index.Tree)
	report.add(DoctorCheck{Name: "index.parse", Status: DoctorPass, Message: fmt.Sprintf("Index OK: %d nodes (%d files, %d dirs)", stats.TotalNodes, stats.TotalFiles, stats.TotalDirs)})
}

func checkCacheFile(report *DoctorReport, cachePath string) {
	if _, err := os.Stat(cachePath); err != nil {
		if os.IsNotExist(err) {
			report.add(DoctorCheck{Name: "cache.exists", Status: DoctorWarn, Message: "Synonym cache not found: " + cachePath, Suggestion: "Run `contexting init` or `watch` to create cache."})
			return
		}
		report.add(DoctorCheck{Name: "cache.exists", Status: DoctorFail, Message: err.Error(), Suggestion: "Check cache path permissions."})
		return
	}

	cache, err := LoadSynonymCache(cachePath)
	if err != nil {
		report.add(DoctorCheck{Name: "cache.parse", Status: DoctorFail, Message: err.Error(), Suggestion: "Delete cache file and let contexting recreate it."})
		return
	}
	report.add(DoctorCheck{Name: "cache.parse", Status: DoctorPass, Message: fmt.Sprintf("Cache OK: %d entries", len(cache))})
}

func checkAPIKey(report *DoctorReport) {
	if _, err := GetAPIKey(); err != nil {
		report.add(DoctorCheck{Name: "openrouter.api_key", Status: DoctorWarn, Message: "OPENROUTER_API_KEY not set", Suggestion: "Set env var or use --api-key for LLM synonym generation."})
		return
	}
	report.add(DoctorCheck{Name: "openrouter.api_key", Status: DoctorPass, Message: "OPENROUTER_API_KEY is set."})
}

func checkWriteAccess(report *DoctorReport, root string) {
	tmp, err := os.CreateTemp(root, ".contexting-doctor-*.tmp")
	if err != nil {
		report.add(DoctorCheck{Name: "root.write", Status: DoctorFail, Message: err.Error(), Suggestion: "Ensure write permission on project root."})
		return
	}
	_ = tmp.Close()
	_ = os.Remove(tmp.Name())
	report.add(DoctorCheck{Name: "root.write", Status: DoctorPass, Message: "Project root is writable."})
}

func (r *DoctorReport) add(check DoctorCheck) {
	r.Checks = append(r.Checks, check)
	if check.Status == DoctorFail {
		r.Healthy = false
	}
}

func (r DoctorReport) toJSON() (string, error) {
	bytes, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func defaultCommonFlags() CommonFlags {
	flags := CommonFlags{}
	flags.normalize()
	return flags
}

func applyCommonConfigNoCLI(flags *CommonFlags, cfg CommonConfig) {
	if cfg.OutputPath != "" {
		flags.OutputPath = cfg.OutputPath
	}
	if cfg.SynonymCache != "" {
		flags.SynonymCache = cfg.SynonymCache
	}
	if cfg.Model != "" {
		flags.Model = cfg.Model
	}
	if cfg.APIKey != "" {
		flags.APIKey = cfg.APIKey
	}
	if cfg.BatchSize > 0 {
		flags.BatchSize = cfg.BatchSize
	}
	if cfg.SynonymsPerName > 0 {
		flags.SynonymsPerName = cfg.SynonymsPerName
	}
	if len(cfg.ExtraIgnores) > 0 {
		flags.ExtraIgnores = append([]string(nil), cfg.ExtraIgnores...)
	}
	if cfg.Verbose != nil {
		flags.Verbose = *cfg.Verbose
	}
}
