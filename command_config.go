package main

import "github.com/spf13/cobra"

func newConfigCommand() *cobra.Command {
	cfgCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage context.toml configuration",
	}
	cfgCmd.AddCommand(newConfigInitCommand())
	return cfgCmd
}

func newConfigInitCommand() *cobra.Command {
	var outPath string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create starter context.toml",
		RunE: func(cmd *cobra.Command, args []string) error {
			target := outPath
			if target == "" {
				target = configPath
			}
			if err := writeStarterConfig(target, force); err != nil {
				return err
			}
			logInfof("Created starter config at %s", target)
			return nil
		},
	}

	cmd.Flags().StringVar(&outPath, "output", "", "Path to write starter config (defaults to --config path)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config file")
	return cmd
}
