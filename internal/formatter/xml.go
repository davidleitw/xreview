package formatter

import "github.com/davidleitw/xreview/internal/session"

// Check represents a preflight check result.
type Check struct {
	Name   string
	Passed bool
	Detail string
}

// FormatReviewResult produces XML output for a review/verify round.
func FormatReviewResult(sessionID string, round int, action string, findings []session.Finding, summary session.FindingSummary) string {
	// TODO: implement
	return ""
}

// FormatPreflightResult produces XML output for a preflight check.
func FormatPreflightResult(checks []Check) string {
	// TODO: implement
	return ""
}

// FormatVersionResult produces XML output for a version check.
func FormatVersionResult(current, latest string, outdated bool) string {
	// TODO: implement
	return ""
}

// FormatReportResult produces XML output for a report generation.
func FormatReportResult(sessionID, path string, summary session.FindingSummary) string {
	// TODO: implement
	return ""
}

// FormatCleanResult produces XML output for a session cleanup.
func FormatCleanResult(sessionID string) string {
	// TODO: implement
	return ""
}
