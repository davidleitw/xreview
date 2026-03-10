package main

import (
	"fmt"
	"strings"

	"github.com/davidleitw/xreview/internal/codex"
	"github.com/davidleitw/xreview/internal/collector"
	"github.com/davidleitw/xreview/internal/config"
	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/parser"
	"github.com/davidleitw/xreview/internal/prompt"
	"github.com/davidleitw/xreview/internal/reviewer"
	"github.com/davidleitw/xreview/internal/session"
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
		contextStr     string
	)

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Run code review or continue existing session",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			hasFiles := files != ""
			hasGit := gitUncommitted
			hasSession := sessionID != ""

			if hasFiles && hasGit {
				return fmt.Errorf("--files and --git-uncommitted are mutually exclusive")
			}
			if !hasSession && !hasFiles && !hasGit {
				return fmt.Errorf("new review requires --files or --git-uncommitted")
			}
			if hasSession && (hasFiles || hasGit) {
				return fmt.Errorf("--files/--git-uncommitted cannot be used with --session")
			}
			if !hasSession && message != "" {
				return fmt.Errorf("--message requires --session")
			}
			if !hasSession && fullRescan {
				return fmt.Errorf("--full-rescan requires --session")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flagWorkdir)
			if err != nil {
				fmt.Println(formatter.FormatError("review", formatter.ErrInvalidFlags, err.Error()))
				return err
			}

			builder, err := prompt.NewBuilder()
			if err != nil {
				return fmt.Errorf("init prompt builder: %w", err)
			}

			rev := reviewer.NewSingleReviewer(
				codex.NewRunner(),
				builder,
				parser.NewParser(),
				session.NewManager(flagWorkdir),
				collector.NewCollector(cfg, flagWorkdir),
				cfg,
			)

			if sessionID != "" {
				// Resume/verify existing session
				result, err := rev.Verify(cmd.Context(), reviewer.VerifyRequest{
					SessionID:  sessionID,
					Message:    message,
					FullRescan: fullRescan,
					Timeout:    timeout,
				})
				if err != nil {
					fmt.Println(formatter.FormatError("review", formatter.ErrCodexError, err.Error()))
					return err
				}
				fmt.Println(formatter.FormatReviewResult(
					result.SessionID, result.Round, result.Verdict,
					result.Findings, result.Summary,
				))
				return nil
			}

			// New review
			var targets []string
			targetMode := "files"

			if gitUncommitted {
				targetMode = "git-uncommitted"
			} else {
				targets = splitTargets(files)
			}

			result, err := rev.Review(cmd.Context(), reviewer.ReviewRequest{
				Targets:    targets,
				TargetMode: targetMode,
				Context:    contextStr,
				Timeout:    timeout,
			})
			if err != nil {
				fmt.Println(formatter.FormatError("review", formatter.ErrCodexError, err.Error()))
				return err
			}

			fmt.Println(formatter.FormatReviewResult(
				result.SessionID, result.Round, result.Verdict,
				result.Findings, result.Summary,
			))
			return nil
		},
	}

	cmd.Flags().StringVar(&files, "files", "", "Comma-separated file or directory paths to review")
	cmd.Flags().BoolVar(&gitUncommitted, "git-uncommitted", false, "Review all uncommitted changes")
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID for continuing a review")
	cmd.Flags().StringVar(&message, "message", "", "Message describing fixes or dismissals")
	cmd.Flags().BoolVar(&fullRescan, "full-rescan", false, "Start fresh codex session for rescan")
	cmd.Flags().IntVar(&timeout, "timeout", 180, "Timeout in seconds for codex response")
	cmd.Flags().StringVar(&contextStr, "context", "", "Structured context describing the change")

	return cmd
}

func splitTargets(s string) []string {
	var targets []string
	for _, t := range strings.Split(s, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			targets = append(targets, t)
		}
	}
	return targets
}
