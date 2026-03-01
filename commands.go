package main

import "github.com/spf13/cobra"

var configPath string
var noConfigPrompt bool
var createConfig bool

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "contexting",
		Short: "Index codebases with synonym hints for AI workflows",
		Long:  "Contexting builds a filesystem index and optional LLM-generated synonyms for improved code search context.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip auto-prompt for help/config management flows.
			if cmd.Name() == "help" || cmd.Name() == "config" || cmd.Parent() != nil && cmd.Parent().Name() == "config" {
				return nil
			}
			if noConfigPrompt {
				return nil
			}
			return ensureStarterConfigPrompt(configPath, createConfig)
		},
	}
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "context.toml", "Path to contexting TOML config")
	rootCmd.PersistentFlags().BoolVar(&noConfigPrompt, "no-config-prompt", false, "Disable interactive starter config prompt when config is missing")
	rootCmd.PersistentFlags().BoolVar(&createConfig, "create-config", false, "Auto-create starter config when missing (non-interactive)")

	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newWatchCommand())
	rootCmd.AddCommand(newSearchCommand())
	rootCmd.AddCommand(newEvalCommand())
	rootCmd.AddCommand(newDoctorCommand())
	rootCmd.AddCommand(newConfigCommand())

	return rootCmd
}
