package contexting

import "github.com/spf13/cobra"

func newConfigCommand() *cobra.Command {
	cfgCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage .ctxt/ctx_config.toml configuration",
	}
	cfgCmd.AddCommand(newConfigInitCommand())
	return cfgCmd
}

func newConfigInitCommand() *cobra.Command {
	var outPath string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create starter .ctxt/ctx_config.toml",
		Long: `Create the project config before indexing so you can choose your model and settings.

From the project root, run "ctxt config init", edit [llm].model in
.ctxt/ctx_config.toml, then run "ctxt init ." to build the index.

Existing config files are preserved unless you pass --force.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := outPath
			if target == "" {
				target = configPath
			}
			if err := writeStarterConfig(target, force); err != nil {
				return err
			}
			LogInfof("Created starter config at %s", target)
			LogInfof("Edit [llm].model and other settings before indexing your project.")
			return nil
		},
	}

	cmd.Flags().StringVar(&outPath, "output", "", "Path to write starter config (defaults to --config path)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config file")
	return cmd
}
