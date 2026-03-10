package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newReviewCmd() *cobra.Command {
	var (
		files          string
		gitUncommitted bool
		sessionID      string
		message        string
		fullRescan     bool
		timeout        int
	)

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Run code review or continue existing session",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			hasFiles := files != ""
			hasGit := gitUncommitted
			hasSession := sessionID != ""

			// --files and --git-uncommitted are mutually exclusive
			if hasFiles && hasGit {
				return fmt.Errorf("--files and --git-uncommitted are mutually exclusive")
			}

			// New review requires --files or --git-uncommitted
			if !hasSession && !hasFiles && !hasGit {
				return fmt.Errorf("new review requires --files or --git-uncommitted")
			}

			// --files/--git-uncommitted cannot combine with --session
			if hasSession && (hasFiles || hasGit) {
				return fmt.Errorf("--files/--git-uncommitted cannot be used with --session")
			}

			// --message and --full-rescan require --session
			if !hasSession && message != "" {
				return fmt.Errorf("--message requires --session")
			}
			if !hasSession && fullRescan {
				return fmt.Errorf("--full-rescan requires --session")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("review: not implemented")
		},
	}

	cmd.Flags().StringVar(&files, "files", "", "Comma-separated file or directory paths to review")
	cmd.Flags().BoolVar(&gitUncommitted, "git-uncommitted", false, "Review all uncommitted changes")
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID for continuing a review")
	cmd.Flags().StringVar(&message, "message", "", "Message describing fixes or dismissals")
	cmd.Flags().BoolVar(&fullRescan, "full-rescan", false, "Start fresh codex session for rescan")
	cmd.Flags().IntVar(&timeout, "timeout", 180, "Timeout in seconds for codex response")

	return cmd
}
