package contexting

import "github.com/spf13/cobra"

func applyCommonConfig(cmd *cobra.Command, flags *CommonFlags, cfg CommonConfig) {
	applyStringFlag(cmd, "output", &flags.OutputPath, cfg.OutputPath)
	applyStringFlag(cmd, "synonym-cache", &flags.SynonymCache, cfg.SynonymCache)
	applyStringFlag(cmd, "llm-model", &flags.Model, cfg.Model)
	applyStringFlag(cmd, "api-key", &flags.APIKey, cfg.APIKey)
	applyIntFlag(cmd, "batch-size", &flags.BatchSize, cfg.BatchSize)
	applyIntFlag(cmd, "synonyms", &flags.SynonymsPerName, cfg.SynonymsPerName)
	applyIntFlag(cmd, "synonyms-min", &flags.SynonymsMin, cfg.SynonymsMin)
	applyIntFlag(cmd, "synonyms-max", &flags.SynonymsMax, cfg.SynonymsMax)
	applyStringFlag(cmd, "symbol-extractor", &flags.SymbolExtractor, cfg.SymbolExtractor)
	applyStringSliceFlag(cmd, "ignore", &flags.ExtraIgnores, cfg.ExtraIgnores)
	if cfg.Verbose != nil {
		applyBoolFlag(cmd, "verbose", &flags.Verbose, *cfg.Verbose)
	}
	// Set the global mode from config when the user didn't pass the flag.
	if cfg.SymbolExtractor != "" && !cmd.Flags().Changed("symbol-extractor") {
		SymbolsExtractorMode = cfg.SymbolExtractor
	}
}

func applyStringFlag(cmd *cobra.Command, name string, target *string, value string) {
	if value == "" || cmd.Flags().Changed(name) {
		return
	}
	*target = value
}

func applyIntFlag(cmd *cobra.Command, name string, target *int, value int) {
	if cmd.Flags().Changed(name) {
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
