package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ensureStarterConfigPrompt(path string, autoCreate bool) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check config path %s: %w", path, err)
	}

	if autoCreate {
		return writeStarterConfig(path, false)
	}

	if !isInteractiveTerminal() {
		return nil
	}

	ok, err := askYesNo(fmt.Sprintf("Config file %q not found. Create starter config now? [Y/n]: ", path), true)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	if err := writeStarterConfig(path, false); err != nil {
		return err
	}
	logInfof("Created starter config at %s", path)
	return nil
}

func writeStarterConfig(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check config path %s: %w", path, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	return writeFileAtomic(path, []byte(starterConfigTemplate), 0o644)
}

func askYesNo(prompt string, defaultYes bool) (bool, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read prompt input: %w", err)
	}
	value := strings.ToLower(strings.TrimSpace(input))
	if value == "" {
		return defaultYes, nil
	}
	if value == "y" || value == "yes" {
		return true, nil
	}
	if value == "n" || value == "no" {
		return false, nil
	}
	return defaultYes, nil
}

func isInteractiveTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
