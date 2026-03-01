package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newEvalCommand() *cobra.Command {
	var indexPath string
	var casesPath string
	var opts SearchOptions
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Evaluate search quality against expected query/path cases",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadContextingConfig(configPath)
			if err != nil {
				return err
			}
			applyStringFlag(cmd, "index", &indexPath, cfg.Eval.IndexPath)
			applyStringFlag(cmd, "cases", &casesPath, cfg.Eval.CasesPath)
			applyIntFlag(cmd, "limit", &opts.Limit, cfg.Eval.Limit)
			applyIntFlag(cmd, "min-score", &opts.MinScore, cfg.Eval.MinScore)
			applyStringFlag(cmd, "type", &opts.TypeFilter, cfg.Eval.TypeFilter)
			if cfg.Eval.Explain != nil {
				applyBoolFlag(cmd, "explain", &opts.IncludeDebug, *cfg.Eval.Explain)
			}
			if cfg.Eval.JSON != nil {
				applyBoolFlag(cmd, "json", &jsonOut, *cfg.Eval.JSON)
			}
			if !cmd.Flags().Changed("index") {
				indexPath = resolveConfigPath(configPath, indexPath)
			}
			if !cmd.Flags().Changed("cases") {
				casesPath = resolveConfigPath(configPath, casesPath)
			}

			if casesPath == "" {
				return fmt.Errorf("--cases is required")
			}

			index, err := LoadContextIndex(indexPath)
			if err != nil {
				return err
			}
			cases, err := LoadEvalCases(casesPath)
			if err != nil {
				return err
			}

			summary, results := EvaluateSearch(index, cases, opts)
			if jsonOut {
				type payload struct {
					Summary EvalSummary  `json:"summary"`
					Cases   []EvalResult `json:"cases"`
				}
				jsonStr, err := resultsToJSONEval(payload{Summary: summary, Cases: results})
				if err != nil {
					return err
				}
				fmt.Println(jsonStr)
				return nil
			}

			printEvalSummary(summary)
			printFailedCases(results)
			return nil
		},
	}

	cmd.Flags().StringVarP(&indexPath, "index", "i", "context.json", "Path to context JSON")
	cmd.Flags().StringVar(&casesPath, "cases", "", "Path to eval cases JSON")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "n", 5, "Number of ranked search results per query")
	cmd.Flags().IntVar(&opts.MinScore, "min-score", 1, "Minimum score required for candidate results")
	cmd.Flags().StringVar(&opts.TypeFilter, "type", "all", "Filter result type: all|files|dirs")
	cmd.Flags().BoolVar(&opts.IncludeDebug, "explain", false, "Include score breakdown in eval case results")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Print full eval report as JSON")

	return cmd
}
