package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDoctorCommand() *cobra.Command {
	var opts DoctorOptions
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check project/config/index health and print actionable fixes",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.ConfigPath = configPath
			report := RunDoctor(opts)

			if jsonOut {
				payload, err := report.toJSON()
				if err != nil {
					return err
				}
				fmt.Println(payload)
				if !report.Healthy {
					return fmt.Errorf("doctor reported failing checks")
				}
				return nil
			}

			printDoctorReport(report)
			if !report.Healthy {
				return fmt.Errorf("doctor reported failing checks")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.RootPath, "root", "", "Project root path override")
	cmd.Flags().StringVar(&opts.IndexPath, "index", "", "Index path override")
	cmd.Flags().StringVar(&opts.CachePath, "synonym-cache", "", "Synonym cache path override")
	cmd.Flags().BoolVar(&opts.WriteCheck, "write-check", true, "Check write access by creating a temporary file")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output doctor report as JSON")

	return cmd
}

func printDoctorReport(report DoctorReport) {
	for _, check := range report.Checks {
		prefix := "[PASS]"
		switch check.Status {
		case DoctorWarn:
			prefix = "[WARN]"
		case DoctorFail:
			prefix = "[FAIL]"
		}
		logInfof("%s %s: %s", prefix, check.Name, check.Message)
		if check.Suggestion != "" {
			logInfof("fix: %s", check.Suggestion)
		}
	}

	if report.Healthy {
		logInfof("Doctor status: healthy")
	} else {
		logWarnf("Doctor status: issues found")
	}
}
