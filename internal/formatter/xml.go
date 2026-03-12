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
// to guide Claude Code on how to verify and present findings to the user.
func buildAgentInstructions(findings []session.Finding, summary session.FindingSummary, sessionID string) string {
	var b strings.Builder

	b.WriteString("<agent-instructions>\n")

	// Phase 1: Verify findings
	b.WriteString("== PHASE 1: VERIFY FINDINGS (BLOCKING — do this BEFORE presenting anything to user) ==\n\n")
	b.WriteString("You are an INDEPENDENT reviewer. Codex findings may contain false positives.\n")
	b.WriteString("You MUST verify each finding by reading the actual code BEFORE building the Fix Plan.\n\n")
	b.WriteString("REQUIRED ACTIONS for EACH finding — you must SHOW YOUR WORK:\n\n")
	b.WriteString("1. USE the Read tool to read the source file at the specified line.\n")
	b.WriteString("   You MUST actually call the Read tool — do NOT rely on memory or assumptions.\n")
	b.WriteString("2. QUOTE the relevant code lines you read (2-5 lines around the issue).\n")
	b.WriteString("3. EXPLAIN in 1-2 sentences why this finding is valid or not, based on what you read.\n")
	b.WriteString("4. CLASSIFY:\n")
	b.WriteString("   - ✅ CONFIRMED — you saw the issue in the actual code\n")
	b.WriteString("   - ❌ SUSPECT — the code does NOT match what Codex described (explain why)\n\n")
	b.WriteString("Present verification results in this format BEFORE the Fix Plan:\n\n")
	b.WriteString("```\n")
	b.WriteString("## Verification Results\n\n")
	b.WriteString("### F-001: [title]\n")
	b.WriteString("Code at file.go:42:\n")
	b.WriteString("  > line of code from Read tool\n")
	b.WriteString("  > line of code from Read tool\n")
	b.WriteString("Analysis: [why this is/isn't a real issue]\n")
	b.WriteString("Verdict: ✅ CONFIRMED / ❌ SUSPECT\n")
	b.WriteString("```\n\n")
	b.WriteString("If you skip the Read tool or present findings without showing the code you read,\n")
	b.WriteString("you are acting as a blind proxy — which defeats the entire purpose of this review.\n\n")
	fmt.Fprintf(&b, "For SUSPECT findings, CHALLENGE Codex before dropping them:\n")
	fmt.Fprintf(&b, "  Run: xreview review --session %s --message \"F-XXX: I read the code at [file:line] and believe this is a false positive because [your reasoning with code evidence]. Please re-evaluate.\"\n", sessionID)
	b.WriteString("  If Codex agrees → drop finding. If Codex provides valid counter-reasoning → mark CONFIRMED.\n")
	b.WriteString("  If disagreement persists → present both perspectives to user.\n\n")

	// Phase 2: Fix Plan
	b.WriteString("== PHASE 2: FIX PLAN (only CONFIRMED findings from Phase 1) ==\n\n")
	b.WriteString("ONLY after completing ALL Phase 1 verifications, present the Fix Plan.\n")
	b.WriteString("Include ONLY findings you marked ✅ CONFIRMED. Drop all ❌ SUSPECT findings\n")
	b.WriteString("(unless Codex challenged your reasoning and you changed your mind).\n\n")
	b.WriteString("This is a hard gate — do NOT start fixing code until user approves.\n\n")

	b.WriteString("For EACH confirmed finding, include:\n")
	b.WriteString("1. Header: ### F-XXX: title (category/severity) with file:line\n")
	b.WriteString("2. Trigger: the trigger condition — as verified by YOUR code reading, not just copied from Codex\n")
	b.WriteString("3. Impact: what happens if exploited/triggered\n")
	b.WriteString("4. Cascade: list every <impact> from <cascade-impact> — these show what else breaks\n")
	b.WriteString("5. Fix options: list ALL <alternative> entries, mark which is recommended.\n")
	b.WriteString("   ALWAYS add a final option: \"Don't fix — risk: <consequence>\"\n\n")

	// Count severities for context-aware guidance.
	var highCount, mediumCount, lowCount int
	for _, f := range findings {
		if f.Status != session.FindingOpen && f.Status != session.FindingReopened {
			continue
		}
		switch f.Severity {
		case "high":
			highCount++
		case "medium":
			mediumCount++
		case "low":
			lowCount++
		}
	}

	if highCount > 0 {
		fmt.Fprintf(&b, "⚠ %d HIGH severity finding(s) — verify with extra care for each.\n", highCount)
	}
	if mediumCount > 0 {
		fmt.Fprintf(&b, "%d MEDIUM severity finding(s) — verify with extra care, can be shorter than high.\n", mediumCount)
	}
	if lowCount > 0 {
		fmt.Fprintf(&b, "%d LOW severity finding(s) — brief description with fix options is sufficient.\n", lowCount)
	}

	b.WriteString("\nAfter listing ALL findings, you MUST use AskUserQuestion with these options:\n")
	fmt.Fprintf(&b, "  A. Execute all recommended fixes\n")
	fmt.Fprintf(&b, "  B. Only fix high severity, skip the rest\n")
	fmt.Fprintf(&b, "  C. I want to adjust (tell me which findings to change)\n")
	b.WriteString("Do NOT proceed until the user responds.\n")
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
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
