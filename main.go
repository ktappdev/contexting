package main

import (
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "contexting",
		Short: "A tool for indexing codebase with synonyms for better LLM context",
		Long:  "Contexting indexes your codebase and generates synonyms to help AI agents find relevant files.",
	}

	watchCmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch the current directory for changes",
		Run: func(cmd *cobra.Command, args []string) {
			watchDir, _ := os.Getwd()
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating watcher: %v\n", err)
				os.Exit(1)
			}
			defer watcher.Close()

			err = watcher.Add(watchDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error adding directory to watcher: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Watching %s for changes...\n", watchDir)

			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if event.Op&fsnotify.Write == fsnotify.Write {
						fmt.Printf("Modified: %s\n", event.Name)
					} else if event.Op&fsnotify.Create == fsnotify.Create {
						fmt.Printf("Created: %s\n", event.Name)
					} else if event.Op&fsnotify.Remove == fsnotify.Remove {
						fmt.Printf("Removed: %s\n", event.Name)
					} else if event.Op&fsnotify.Rename == fsnotify.Rename {
						fmt.Printf("Renamed: %s\n", event.Name)
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
				}
			}
		},
	}

	rootCmd.AddCommand(watchCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
