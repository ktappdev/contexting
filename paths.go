package main

import "path/filepath"

func resolveProjectPath(projectRoot string, path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	if projectRoot == "" {
		return path
	}
	return filepath.Join(projectRoot, path)
}

func resolveConfigPath(configFile string, path string) string {
	if path == "" || filepath.IsAbs(path) || configFile == "" {
		return path
	}
	return filepath.Join(filepath.Dir(configFile), path)
}
