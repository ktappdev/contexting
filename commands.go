package contexting

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
		Version: Version,
		Short:   "ctxt — concept-based code search for AI-assisted development",
		Long: `ctxt builds a searchable index of your codebase — files, symbols (functions/classes/types), and LLM-generated synonyms — letting AI assistants and CLI users find files by what they DO, not what they're named.

Key features:
  • Tree-sitter AST extraction — captures nested methods/functions in Python, JS/TS, Rust, Svelte, Astro
  • Import-aware synonyms — LLM sees ESM imports to generate domain-accurate names (e.g., "clerk webhook handler" for route.ts)
  • Hybrid search — ripgrep content fallback when index results are sparse
  • MCP server — integrate with Claude Desktop, Cursor, and other AI assistants
  • 7 bench engines — compare ctxt against find, fd, grep, rg

Get started:
  ctxt init .           # Build index for current project
  ctxt watch .          # Watch for changes, keep index live
  ctxt mcp              # Start MCP server for AI assistants
  ctxt search-hints "auth middleware"  # Find auth-related files
  ctxt examples         # See more usage patterns`,
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

	// Command groups for organized help output
	rootCmd.AddGroup(&cobra.Group{ID: "setup", Title: "Setup:"})
	rootCmd.AddGroup(&cobra.Group{ID: "query", Title: "Query:"})
	rootCmd.AddGroup(&cobra.Group{ID: "analysis", Title: "Analysis:"})
	rootCmd.AddGroup(&cobra.Group{ID: "maintenance", Title: "Maintenance:"})
	rootCmd.AddGroup(&cobra.Group{ID: "other", Title: "Other:"})

	// Setup
	initCmd := newInitCommand()
	initCmd.GroupID = "setup"
	rootCmd.AddCommand(initCmd)

	watchCmd := newWatchCommand()
	watchCmd.GroupID = "setup"
	rootCmd.AddCommand(watchCmd)

	syncCmd := newSyncCommand()
	syncCmd.GroupID = "setup"
	rootCmd.AddCommand(syncCmd)

	// Query
	mcpCmd := newMCPCommand()
	mcpCmd.GroupID = "query"
	rootCmd.AddCommand(mcpCmd)

	searchCmd := newSearchCommand()
	searchCmd.GroupID = "query"
	rootCmd.AddCommand(searchCmd)

	// Analysis
	benchCmd := newBenchCommand()
	benchCmd.GroupID = "analysis"
	rootCmd.AddCommand(benchCmd)

	evalCmd := newEvalCommand()
	evalCmd.GroupID = "analysis"
	rootCmd.AddCommand(evalCmd)

	// Maintenance
	doctorCmd := newDoctorCommand()
	doctorCmd.GroupID = "maintenance"
	rootCmd.AddCommand(doctorCmd)

	statusCmd := newStatusCommand()
	statusCmd.GroupID = "maintenance"
	rootCmd.AddCommand(statusCmd)

	configCmd := newConfigCommand()
	configCmd.GroupID = "maintenance"
	rootCmd.AddCommand(configCmd)

	cleanCmd := newCleanCommand()
	cleanCmd.GroupID = "maintenance"
	rootCmd.AddCommand(cleanCmd)

	// Other
	examplesCmd := newExamplesCommand()
	examplesCmd.GroupID = "other"
	rootCmd.AddCommand(examplesCmd)

	return rootCmd
}
