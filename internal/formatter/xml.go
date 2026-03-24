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
func FormatReviewResult(sessionID string, round int, verdict string, findings []session.Finding, summary session.FindingSummary, language string) string {
	var b strings.Builder

	if language != "" {
		fmt.Fprintf(&b, `<xreview-result status="success" action="review" session="%s" round="%d" language="%s">`+"\n",
			xmlEscape(sessionID), round, xmlEscape(language))
	} else {
		fmt.Fprintf(&b, `<xreview-result status="success" action="review" session="%s" round="%d">`+"\n",
			xmlEscape(sessionID), round)
	}

	fmt.Fprintf(&b, `  <verdict>%s</verdict>`+"\n", xmlEscape(verdict))

	for _, f := range findings {
		fmt.Fprintf(&b, `  <finding id="%s" severity="%s" category="%s" status="%s" confidence="%d" fix-strategy="%s">`+"\n",
			xmlEscape(f.ID), xmlEscape(f.Severity), xmlEscape(f.Category), xmlEscape(f.Status), f.Confidence, xmlEscape(f.FixStrategy))
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
		if f.Trigger != "" {
			fmt.Fprintf(&b, "    <trigger>%s</trigger>\n", xmlEscape(f.Trigger))
		}
		if len(f.CascadeImpact) > 0 {
			b.WriteString("    <cascade-impact>\n")
			for _, ci := range f.CascadeImpact {
				fmt.Fprintf(&b, "      <impact>%s</impact>\n", xmlEscape(ci))
			}
			b.WriteString("    </cascade-impact>\n")
		}
		if len(f.FixAlternatives) > 0 {
			b.WriteString("    <fix-alternatives>\n")
			for _, alt := range f.FixAlternatives {
				fmt.Fprintf(&b, "      <alternative label=\"%s\" effort=\"%s\" recommended=\"%t\">%s</alternative>\n",
					xmlEscape(alt.Label), xmlEscape(alt.Effort), alt.Recommended, xmlEscape(alt.Description))
			}
			b.WriteString("    </fix-alternatives>\n")
		}
		b.WriteString("  </finding>\n")
	}

	fmt.Fprintf(&b, `  <summary total="%d" open="%d" fixed="%d" dismissed="%d" />`+"\n",
		summary.Total, summary.Open, summary.Fixed, summary.Dismissed)

	b.WriteString("</xreview-result>")

	// Append agent instructions when there are open findings to guide Claude Code behavior.
	if summary.Open > 0 {
		b.WriteString("\n\n")
		b.WriteString(buildAgentInstructions(findings, summary, sessionID))
	}

	return b.String()
}

// buildAgentInstructions generates inline instructions appended after XML output
// to guide Claude Code on how to process findings using its skill instructions.
func buildAgentInstructions(findings []session.Finding, summary session.FindingSummary, sessionID string) string {
	var b strings.Builder

	b.WriteString("<agent-instructions>\n")
	fmt.Fprintf(&b, "Session ID: %s\n", sessionID)
	fmt.Fprintf(&b, "Findings: %d total (%d open, %d fixed, %d dismissed)\n",
		summary.Total, summary.Open, summary.Fixed, summary.Dismissed)

	var highCount, autoCount int
	for _, f := range findings {
		if f.Status != session.FindingOpen && f.Status != session.FindingReopened {
			continue
		}
		if f.Severity == "high" {
			highCount++
		}
		if f.FixStrategy == "auto" {
			autoCount++
		}
	}
	if highCount > 0 {
		fmt.Fprintf(&b, "High severity: %d\n", highCount)
	}
	fmt.Fprintf(&b, "Auto-fixable: %d, Needs discussion: %d\n", autoCount, summary.Open-autoCount)

	b.WriteString("\nFollow the workflow defined in your skill instructions.\n")
	fmt.Fprintf(&b, "Use session ID %s for any xreview review --session commands.\n", sessionID)
	b.WriteString("</agent-instructions>")

	return b.String()
}

// FormatPreflightResult produces XML output for a preflight check.
func FormatPreflightResult(checks []Check, currentVersion, latestVersion string, updateAvailable bool) string {
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
	if latestVersion != "" {
		fmt.Fprintf(&b, `  <version current="%s" latest="%s" update-available="%t" />`+"\n",
			xmlEscape(currentVersion), xmlEscape(latestVersion), updateAvailable)
	} else {
		fmt.Fprintf(&b, `  <version current="%s" />`+"\n", xmlEscape(currentVersion))
	}
	b.WriteString("</xreview-result>")

	return b.String()
}

// FormatSelfUpdateResult produces XML output for a successful self-update.
func FormatSelfUpdateResult(newVersion string) string {
	return fmt.Sprintf(
		`<xreview-result status="success" action="self-update">`+"\n"+
			`  <version new="%s" />` +"\n"+
			`</xreview-result>`,
		xmlEscape(newVersion),
	)
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

// FormatCleanResult produces XML output for a session cleanup.
func FormatCleanResult(sessionID string) string {
	return fmt.Sprintf(
		`<xreview-result status="success" action="clean" session="%s">`+"\n"+
			`  <message>Session %s deleted successfully.</message>`+"\n"+
			`</xreview-result>`,
		xmlEscape(sessionID), xmlEscape(sessionID),
	)
}

// FormatCleanAllResult produces XML output for cleaning all sessions.
func FormatCleanAllResult() string {
	return `<xreview-result status="success" action="clean">` + "\n" +
		`  <message>All sessions deleted successfully.</message>` + "\n" +
		`</xreview-result>`
}

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
