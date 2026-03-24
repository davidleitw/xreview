package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/davidleitw/xreview/internal/config"
	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/session"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var (
		sessionID string
		all       bool
	)

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Delete session data",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" && !all {
				return fmt.Errorf("--session or --all is required")
			}
			if sessionID != "" && all {
				return fmt.Errorf("--session and --all are mutually exclusive")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				sessDir := config.SessionsDir()
				if err := os.RemoveAll(sessDir); err != nil {
					fmt.Println(formatter.FormatError("clean", classifyCleanError(err), err.Error()))
					return err
				}
				fmt.Println(formatter.FormatCleanAllResult())
				return nil
			}

			mgr := session.NewManager()

			if _, err := mgr.Load(sessionID); err != nil {
				fmt.Println(formatter.FormatError("clean", classifyCleanError(err), err.Error()))
				return err
			}

			if err := mgr.Delete(sessionID); err != nil {
				fmt.Println(formatter.FormatError("clean", classifyCleanError(err), err.Error()))
				return err
			}

			fmt.Println(formatter.FormatCleanResult(sessionID))
			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to delete")
	cmd.Flags().BoolVar(&all, "all", false, "Delete all session data")

	return cmd
}

func classifyCleanError(err error) string {
	if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "not found") {
		return formatter.ErrSessionNotFound
	}
	if errors.Is(err, os.ErrPermission) {
		return formatter.ErrIOError
	}
	if strings.Contains(err.Error(), "invalid session ID") {
		return formatter.ErrInvalidFlags
	}
	return formatter.ErrIOError
}
