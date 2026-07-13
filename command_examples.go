package contexting

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newExamplesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "examples",
		Short: "Show common usage patterns",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(`Setup:
  ctxt init .                              Build index for current project
  ctxt init . --model openai/gpt-4o        Use specific model for synonyms

Search:
  ctxt search-hints "auth"                 Find auth-related files
  ctxt search-hints "payment processing"   --json     JSON output
  ctxt search-hints "login"                --hybrid   With ripgrep content fallback
  ctxt search-hints "database connection"  --limit 20  More results

Development:
  ctxt watch .                             Watch for changes, keep index live
  ctxt sync                                Regenerate synonyms for changed files

AI Assistant:
  ctxt mcp                                 Start MCP server (for Claude/Cursor)
  ctxt mcp --symbol-extractor treesitter   Force tree-sitter extraction

Benchmarks:
  ctxt bench --cases docs/bench_cases.json                          Run benchmark suite
  ctxt bench --cases bench.json --json --output results.json        JSON output

Inspection:
  ctxt status                    Show index stats
  ctxt doctor                    Health check
  ctxt config show               View current config
  ctxt clean                     Remove all ctxt files
`)
		},
	}
}
