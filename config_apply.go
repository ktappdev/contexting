package main

import "github.com/spf13/cobra"

func applyCommonConfig(cmd *cobra.Command, flags *CommonFlags, cfg CommonConfig) {
	applyStringFlag(cmd, "output", &flags.OutputPath, cfg.OutputPath)
	applyStringFlag(cmd, "synonym-cache", &flags.SynonymCache, cfg.SynonymCache)
	applyStringFlag(cmd, "llm-model", &flags.Model, cfg.Model)
	applyStringFlag(cmd, "api-key", &flags.APIKey, cfg.APIKey)
	applyIntFlag(cmd, "batch-size", &flags.BatchSize, cfg.BatchSize)
	applyIntFlag(cmd, "synonyms", &flags.SynonymsPerName, cfg.SynonymsPerName)
	applyStringSliceFlag(cmd, "ignore", &flags.ExtraIgnores, cfg.ExtraIgnores)
	if cfg.Verbose != nil {
		applyBoolFlag(cmd, "verbose", &flags.Verbose, *cfg.Verbose)
	}
}

func applyStringFlag(cmd *cobra.Command, name string, target *string, value string) {
	if value == "" || cmd.Flags().Changed(name) {
		return
	}
	*target = value
}

func applyIntFlag(cmd *cobra.Command, name string, target *int, value int) {
	if value <= 0 || cmd.Flags().Changed(name) {
		return
	}
	*target = value
}

func applyBoolFlag(cmd *cobra.Command, name string, target *bool, value bool) {
	if cmd.Flags().Changed(name) {
		return
	}
	*target = value
}

func applyStringSliceFlag(cmd *cobra.Command, name string, target *[]string, value []string) {
	if len(value) == 0 || cmd.Flags().Changed(name) {
		return
	}
	copied := make([]string, len(value))
	copy(copied, value)
	*target = copied
}
