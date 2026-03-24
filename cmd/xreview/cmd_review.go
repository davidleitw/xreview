package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
		timeout        string
		contextStr     string
		language       string
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
			if !hasSession && message != "" {
				return fmt.Errorf("--message requires --session")
			}
			if !hasSession && fullRescan {
				return fmt.Errorf("--full-rescan requires --session")
			}

			if language != "" {
				if _, ok := prompt.SupportedLanguages[language]; !ok {
					return fmt.Errorf("unsupported language %q; supported: %s",
						language, prompt.SupportedLanguageList())
				}
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			timeoutSecs, err := parseDuration(timeout)
			if err != nil {
				return printErr("review", formatter.ErrInvalidFlags, err)
			}

			cfg, err := config.Load(flagWorkdir)
			if err != nil {
				return printErr("review", formatter.ErrInvalidFlags, err)
			}

			builder, err := prompt.NewBuilder()
			if err != nil {
				return printErr("review", formatter.ErrCodexError, err)
			}

			rev := reviewer.NewSingleReviewer(
				codex.NewRunner(),
				builder,
				parser.NewParser(),
				session.NewManager(),
				collector.NewCollector(cfg, flagWorkdir),
				cfg,
				flagWorkdir,
			)

			if sessionID != "" {
				req := reviewer.VerifyRequest{
					SessionID:  sessionID,
					Message:    message,
					FullRescan: fullRescan,
					Timeout:    timeoutSecs,
				}

				// Pass extra files/git targets if provided
				if gitUncommitted {
					req.ExtraTargetMode = "git-uncommitted"
				} else if files != "" {
					req.ExtraTargets = splitTargets(files)
					req.ExtraTargetMode = "files"
				}

				result, err := rev.Verify(cmd.Context(), req)
				if err != nil {
					return printErr("review", classifyReviewError(err), err)
				}
				fmt.Println(formatter.FormatReviewResult(
					result.SessionID, result.Round, result.Verdict,
					result.Findings, result.Summary, result.Language,
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
				Timeout:    timeoutSecs,
				Language:   language,
			})
			if err != nil {
				return printErr("review", classifyReviewError(err), err)
			}

			fmt.Println(formatter.FormatReviewResult(
				result.SessionID, result.Round, result.Verdict,
				result.Findings, result.Summary, result.Language,
			))
			return nil
		},
	}

	cmd.Flags().StringVar(&files, "files", "", "Comma-separated file or directory paths to review")
	cmd.Flags().BoolVar(&gitUncommitted, "git-uncommitted", false, "Review all uncommitted changes")
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID for continuing a review")
	cmd.Flags().StringVar(&message, "message", "", "Message describing fixes or dismissals")
	cmd.Flags().BoolVar(&fullRescan, "full-rescan", false, "Start fresh codex session for rescan")
	cmd.Flags().StringVar(&timeout, "timeout", "10m", "Timeout for codex response (e.g. 5m, 10m30s, 300)")
	cmd.Flags().StringVar(&contextStr, "context", "", "Structured context describing the change")
	cmd.Flags().StringVar(&language, "language", "", "Language-specific review guidelines (e.g. cpp)")

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

// parseDuration parses a timeout string. Accepts Go duration format ("5m", "10m30s")
// or plain integer (treated as seconds for backward compatibility).
func parseDuration(s string) (int, error) {
	if secs, err := strconv.Atoi(s); err == nil {
		if secs <= 0 {
			return 0, fmt.Errorf("timeout must be positive, got %d", secs)
		}
		return secs, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: use seconds (e.g. 300) or duration (e.g. 5m, 10m30s)", s)
	}
	secs := int(d.Seconds())
	if secs <= 0 {
		return 0, fmt.Errorf("timeout must be positive, got %s", s)
	}
	return secs, nil
}

// classifyReviewError maps reviewer errors to the appropriate error code
// so Claude Code can understand and relay the issue to the user.
func classifyReviewError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "codex CLI is not installed"):
		return formatter.ErrCodexNotFound
	case strings.Contains(msg, "\" not found"):
		return formatter.ErrSessionNotFound
	case strings.Contains(msg, "no files to review"):
		return formatter.ErrNoTargets
	case strings.Contains(msg, "file not found") || strings.Contains(msg, "no such file"):
		return formatter.ErrFileNotFound
	case strings.Contains(msg, "not a git repository"):
		return formatter.ErrNotGitRepo
	case strings.Contains(msg, "did not respond within"):
		return formatter.ErrCodexTimeout
	case strings.Contains(msg, "parse codex output"):
		return formatter.ErrParseFailure
	default:
		return formatter.ErrCodexError
	}
}

// printErr outputs a formatted XML error and returns the error for cobra.
func printErr(action, code string, err error) error {
	fmt.Println(formatter.FormatError(action, code, err.Error()))
	return err
}
