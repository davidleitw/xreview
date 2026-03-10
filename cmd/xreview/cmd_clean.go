package main

import (
	"fmt"

	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/session"
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
			mgr := session.NewManager(flagWorkdir)

			// Verify session exists before deleting
			if _, err := mgr.Load(sessionID); err != nil {
				fmt.Println(formatter.FormatError("clean", formatter.ErrSessionNotFound, err.Error()))
				return err
			}

			if err := mgr.Delete(sessionID); err != nil {
				fmt.Println(formatter.FormatError("clean", formatter.ErrSessionNotFound, err.Error()))
				return err
			}

			fmt.Println(formatter.FormatCleanResult(sessionID))
			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to delete")

	return cmd
}
