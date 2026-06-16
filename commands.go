package main

import "github.com/spf13/cobra"

var configPath string
var noConfigPrompt bool
var createConfig bool
var configJustCreated bool

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "contexting",
		Version: version,
		Short: "Index codebases with code symbols and LLM-generated synonyms for AI context",
		Long:  "Contexting builds a filesystem index of code symbols (functions, classes, types, variables) and optional LLM-generated synonyms for improved code search context. Supports Go, Python, JavaScript, TypeScript, Rust, and Ruby.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip auto-prompt for help/config management flows.
			if cmd.Name() == "help" || cmd.Name() == "config" || cmd.Name() == "clean" || cmd.Parent() != nil && cmd.Parent().Name() == "config" {
				return nil
			}
			if noConfigPrompt {
				return nil
			}
			created, err := ensureStarterConfigPrompt(configPath, createConfig)
			configJustCreated = created
			return err
		},
	}
	rootCmd.PersistentFlags().StringVar(&configPath, "config", ".ctx/ctx_config.toml", "Path to contexting TOML config")
	rootCmd.PersistentFlags().BoolVar(&noConfigPrompt, "no-config-prompt", false, "Disable interactive starter config prompt when config is missing")
	rootCmd.PersistentFlags().BoolVar(&createConfig, "create-config", false, "Auto-create starter config when missing (non-interactive)")

	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newWatchCommand())
	rootCmd.AddCommand(newSearchCommand())
	rootCmd.AddCommand(newEvalCommand())
	rootCmd.AddCommand(newDoctorCommand())
	rootCmd.AddCommand(newConfigCommand())
	rootCmd.AddCommand(newSyncCommand())
	rootCmd.AddCommand(newCleanCommand())

	return rootCmd
}
