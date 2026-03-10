package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidleitw/xreview/internal/config"
	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/session"
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
			mgr := session.NewManager(flagWorkdir)

			sess, err := mgr.Load(sessionID)
			if err != nil {
				fmt.Println(formatter.FormatError("report", formatter.ErrSessionNotFound, err.Error()))
				return err
			}

			// Generate report
			report := generateReport(sess)

			// Write report file
			reportDir := filepath.Join(flagWorkdir, config.XReviewDirName, "reports")
			if err := os.MkdirAll(reportDir, 0o755); err != nil {
				return fmt.Errorf("create report dir: %w", err)
			}

			reportPath := filepath.Join(reportDir, sessionID+".json")
			data, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal report: %w", err)
			}

			if err := os.WriteFile(reportPath, data, 0o644); err != nil {
				return fmt.Errorf("write report: %w", err)
			}

			summary := sess.Summarize()
			fmt.Println(formatter.FormatReportResult(sessionID, reportPath, summary))
			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to generate report for")

	return cmd
}

type report struct {
	SessionID string            `json:"session_id"`
	Round     int               `json:"round"`
	Status    string            `json:"status"`
	Targets   []string          `json:"targets"`
	Findings  []session.Finding `json:"findings"`
	Summary   session.FindingSummary `json:"summary"`
}

func generateReport(sess *session.Session) *report {
	return &report{
		SessionID: sess.SessionID,
		Round:     sess.Round,
		Status:    sess.Status,
		Targets:   sess.Targets,
		Findings:  sess.Findings,
		Summary:   sess.Summarize(),
	}
}
