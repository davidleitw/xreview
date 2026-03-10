package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Delete session data",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("clean: not implemented")
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to delete")

	return cmd
}
