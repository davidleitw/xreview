package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newReportCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate review report",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("report: not implemented")
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to generate report for")

	return cmd
}
