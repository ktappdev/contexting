package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newCleanCommand() *cobra.Command {
	var dryRun bool
	var rootPath string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove .ctx/ directory and all contexting files from a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			absRoot, err := filepath.Abs(rootPath)
			if err != nil {
				return fmt.Errorf("resolve root path: %w", err)
			}
			ctxDir := filepath.Join(absRoot, ".ctx")
			info, err := os.Stat(ctxDir)
			if os.IsNotExist(err) {
				logInfof("No .ctx/ directory found at %s — nothing to clean.", ctxDir)
				return nil
			}
			if err != nil {
				return fmt.Errorf("check .ctx directory: %w", err)
			}
			if !info.IsDir() {
				return fmt.Errorf("%s exists but is not a directory", ctxDir)
			}

			if dryRun {
				entries, err := os.ReadDir(ctxDir)
				if err != nil {
					return fmt.Errorf("read .ctx directory: %w", err)
				}
				logInfof("Would remove %s (%d entries):", ctxDir, len(entries))
				for _, e := range entries {
					fmt.Printf("  %s\n", e.Name())
				}
				return nil
			}

			if err := os.RemoveAll(ctxDir); err != nil {
				return fmt.Errorf("remove .ctx directory: %w", err)
			}
			logInfof("Removed %s", ctxDir)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be removed without deleting")
	cmd.Flags().StringVar(&rootPath, "root", ".", "Project root path")

	return cmd
}
