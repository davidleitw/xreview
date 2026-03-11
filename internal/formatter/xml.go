package formatter

import (
	"fmt"
	"strings"

	"github.com/davidleitw/xreview/internal/session"
)

// Check represents a preflight check result.
type Check struct {
	Name   string
	Passed bool
	Detail string
}

// FormatReviewResult produces XML output for a review/verify round.
func FormatReviewResult(sessionID string, round int, verdict string, findings []session.Finding, summary session.FindingSummary) string {
	var b strings.Builder

	fmt.Fprintf(&b, `<xreview-result status="success" action="review" session="%s" round="%d">`+"\n",
		xmlEscape(sessionID), round)

	fmt.Fprintf(&b, `  <verdict>%s</verdict>`+"\n", xmlEscape(verdict))

	for _, f := range findings {
		fmt.Fprintf(&b, `  <finding id="%s" severity="%s" category="%s" status="%s">`+"\n",
			xmlEscape(f.ID), xmlEscape(f.Severity), xmlEscape(f.Category), xmlEscape(f.Status))
		fmt.Fprintf(&b, `    <location file="%s" line="%d" />`+"\n",
			xmlEscape(f.File), f.Line)
		fmt.Fprintf(&b, `    <description>%s</description>`+"\n", xmlEscape(f.Description))
		if f.Suggestion != "" {
			fmt.Fprintf(&b, `    <suggestion>%s</suggestion>`+"\n", xmlEscape(f.Suggestion))
		}
		if f.CodeSnippet != "" {
			fmt.Fprintf(&b, `    <code-snippet>%s</code-snippet>`+"\n", xmlEscape(f.CodeSnippet))
		}
		if f.VerificationNote != "" {
			fmt.Fprintf(&b, `    <verification>%s</verification>`+"\n", xmlEscape(f.VerificationNote))
		}
		b.WriteString("  </finding>\n")
	}

	fmt.Fprintf(&b, `  <summary total="%d" open="%d" fixed="%d" dismissed="%d" />`+"\n",
		summary.Total, summary.Open, summary.Fixed, summary.Dismissed)

	b.WriteString("</xreview-result>")

	return b.String()
}

// FormatPreflightResult produces XML output for a preflight check.
func FormatPreflightResult(checks []Check, version string) string {
	allPassed := true
	for _, c := range checks {
		if !c.Passed {
			allPassed = false
			break
		}
	}

	status := "success"
	if !allPassed {
		status = "error"
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<xreview-result status="%s" action="preflight">`+"\n", status)
	b.WriteString("  <checks>\n")
	for _, c := range checks {
		fmt.Fprintf(&b, `    <check name="%s" passed="%t" detail="%s" />`+"\n",
			xmlEscape(c.Name), c.Passed, xmlEscape(c.Detail))
	}
	b.WriteString("  </checks>\n")
	fmt.Fprintf(&b, `  <version current="%s" />`+"\n", xmlEscape(version))
	b.WriteString("</xreview-result>")

	return b.String()
}

// FormatVersionResult produces XML output for a version check.
func FormatVersionResult(current, latest string, outdated bool) string {
	updateCmd := ""
	if outdated {
		updateCmd = "xreview self-update"
	}

	return fmt.Sprintf(
		`<xreview-result status="success" action="version">`+"\n"+
			`  <version current="%s" latest="%s" outdated="%t" update-command="%s" />`+"\n"+
			`</xreview-result>`,
		xmlEscape(current), xmlEscape(latest), outdated, xmlEscape(updateCmd),
	)
}

// FormatReportResult produces XML output for a report generation.
func FormatReportResult(sessionID, path string, summary session.FindingSummary) string {
	return fmt.Sprintf(
		`<xreview-result status="success" action="report" session="%s">`+"\n"+
			`  <report path="%s" />`+"\n"+
			`  <summary total="%d" open="%d" fixed="%d" dismissed="%d" />`+"\n"+
			`</xreview-result>`,
		xmlEscape(sessionID), xmlEscape(path),
		summary.Total, summary.Open, summary.Fixed, summary.Dismissed,
	)
}

// FormatCleanResult produces XML output for a session cleanup.
func FormatCleanResult(sessionID string) string {
	return fmt.Sprintf(
		`<xreview-result status="success" action="clean" session="%s">`+"\n"+
			`  <message>Session %s deleted successfully.</message>`+"\n"+
			`</xreview-result>`,
		xmlEscape(sessionID), xmlEscape(sessionID),
	)
}

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
