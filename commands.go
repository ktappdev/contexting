package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configPath string
var noConfigPrompt bool
var createConfig bool
var configJustCreated bool
var agentMode bool
var logToStderr bool

// agentUnsafeCommands lists commands that modify project state.
// When --agent is set, these are blocked to prevent accidental damage.
var agentUnsafeCommands = map[string]bool{
	"init":   true,
	"watch":  true,
	"sync":   true,
	"clean":  true,
	"mcp":    true,
}

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "ctxt",
		Version: version,
		Short: "Index codebases with code symbols and LLM-generated synonyms for AI context",
		Long:  "Contexting (command: ctxt) builds a filesystem index of code symbols (functions, classes, types, variables) and optional LLM-generated synonyms for improved code search context. Supports Go, Python, JavaScript, TypeScript, Rust, and Ruby.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip auto-prompt for help/config management flows.
			if agentMode && agentUnsafeCommands[cmd.Name()] {
				return fmt.Errorf("command %q is not available in agent mode; run without --agent or use a human-approved tool", cmd.Name())
			}
			if cmd.Name() == "help" || cmd.Name() == "config" || cmd.Name() == "clean" || cmd.Name() == "status" || cmd.Name() == "mcp" || cmd.Parent() != nil && cmd.Parent().Name() == "config" {
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
	rootCmd.PersistentFlags().StringVar(&configPath, "config", ".ctxt/ctx_config.toml", "Path to ctxt TOML config")
	rootCmd.PersistentFlags().BoolVar(&noConfigPrompt, "no-config-prompt", false, "Disable interactive starter config prompt when config is missing")
	rootCmd.PersistentFlags().BoolVar(&createConfig, "create-config", false, "Auto-create starter config when missing (non-interactive)")
	rootCmd.PersistentFlags().BoolVar(&agentMode, "agent", false, "Agent mode: blocks state-modifying commands (init/watch/sync/clean)")

	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newWatchCommand())
	rootCmd.AddCommand(newSearchCommand())
	rootCmd.AddCommand(newEvalCommand())
	rootCmd.AddCommand(newBenchCommand())
	rootCmd.AddCommand(newDoctorCommand())
	rootCmd.AddCommand(newConfigCommand())
	rootCmd.AddCommand(newSyncCommand())
	rootCmd.AddCommand(newCleanCommand())
	rootCmd.AddCommand(newStatusCommand())
	rootCmd.AddCommand(newMCPCommand())

	return rootCmd
}
