package main

import (
	"fmt"
	"os"

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
			fmt.Println("Watching current directory...")
		},
	}

	rootCmd.AddCommand(watchCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
