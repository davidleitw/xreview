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
// to guide Claude Code on how to verify, present, and (on user request) fix findings.
//
// The flow is: verify → present all findings → wait for user → discussion/fix on demand.
func buildAgentInstructions(findings []session.Finding, summary session.FindingSummary, sessionID string) string {
	var b strings.Builder

	b.WriteString("<agent-instructions>\n")

	// ── Phase 1: Verify findings ──
	b.WriteString("== PHASE 1: VERIFY FINDINGS (BLOCKING — do this BEFORE presenting anything to user) ==\n\n")
	b.WriteString("You are an INDEPENDENT reviewer. Codex findings may contain false positives.\n")
	b.WriteString("You MUST verify each finding by reading the actual code BEFORE presenting to the user.\n\n")
	b.WriteString("STRATEGY: Group findings by file. Read each file ONCE, then verify all findings in that file together.\n\n")
	b.WriteString("For EACH finding, SHOW YOUR WORK:\n\n")
	b.WriteString("1. USE the Read tool to read the source file (once per file, not once per finding).\n")
	b.WriteString("2. QUOTE the relevant code lines (2-5 lines around the issue).\n")
	b.WriteString("3. EXPLAIN in 1-2 sentences why this finding is valid or not.\n")
	b.WriteString("4. CLASSIFY:\n")
	b.WriteString("   - ✅ CONFIRMED — you saw the issue in the actual code\n")
	b.WriteString("   - ❌ SUSPECT — the code does NOT match what Codex described (explain why)\n\n")
	b.WriteString("If you skip the Read tool or present findings without showing the code you read,\n")
	b.WriteString("you are acting as a blind proxy — which defeats the entire purpose of this review.\n\n")
	fmt.Fprintf(&b, "For SUSPECT findings, CHALLENGE Codex before dropping them:\n")
	fmt.Fprintf(&b, "  Run: xreview review --session %s --message \"F-XXX: I read the code at [file:line] and believe this is a false positive because [your reasoning with code evidence]. Please re-evaluate.\"\n", sessionID)
	b.WriteString("  If Codex agrees → drop finding. If Codex provides valid counter-reasoning → mark CONFIRMED.\n")
	b.WriteString("  If disagreement persists → present both perspectives to user.\n\n")

	// ── Phase 2: Present all confirmed findings ──
	b.WriteString("== PHASE 2: PRESENT ALL CONFIRMED FINDINGS ==\n\n")
	b.WriteString("After completing ALL Phase 1 verifications, present every CONFIRMED finding to the user.\n")
	b.WriteString("Drop all ❌ SUSPECT findings (unless Codex challenged your reasoning and you changed your mind).\n\n")

	b.WriteString("For EACH confirmed finding:\n")
	b.WriteString("1. Header: ### F-XXX [SEVERITY/category] title\n")
	b.WriteString("2. Location: 📍 file:line\n")
	b.WriteString("3. Code: quote the code you read in Phase 1\n")
	b.WriteString("4. Trigger: the condition that triggers this issue (from YOUR verification)\n")
	b.WriteString("5. Impact: what happens if exploited/triggered\n")
	b.WriteString("6. Cascade: list cascade impacts (what else breaks)\n")
	b.WriteString("7. Suggested fixes: ALL alternatives with effort levels, mark recommended\n\n")

	// Severity counts for context.
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

	b.WriteString("\nAfter presenting ALL findings, tell the user:\n")
	b.WriteString("\"以上是本次 review 的發現。你可以：\n")
	b.WriteString("- 討論任何 finding 的細節\n")
	b.WriteString("- 告訴我要修哪些（例如「修 F-001 和 F-003」）\n")
	b.WriteString("- 如果有其他想讓 Codex 檢查的方向，也可以告訴我\"\n\n")
	b.WriteString("STOP here. Do NOT proceed to fix anything. Wait for user response.\n\n")

	// ── Phase 3: Discussion & fix guidance ──
	b.WriteString("== PHASE 3: DISCUSSION & FIX GUIDANCE ==\n\n")
	b.WriteString("After presenting findings, handle user messages based on their intent:\n\n")
	b.WriteString("- User asks about a finding → read code and explain in detail.\n")
	b.WriteString("- User challenges a finding → re-verify against code, update classification if wrong.\n")
	fmt.Fprintf(&b, "- User asks Codex to check something new → run: xreview review --session %s --message \"<user's question>\"\n", sessionID)
	b.WriteString("  Then verify any new findings (Phase 1 process) and present them (Phase 2 format).\n")
	b.WriteString("- User requests fixes (e.g. \"修 F-001 和 F-003\") →\n")
	b.WriteString("  1. Apply fixes for the specified findings only.\n")
	fmt.Fprintf(&b, "  2. Verify: xreview review --session %s --message \"Fixed [IDs]. Dismissed [IDs] (reasons). Verify fixes and check for new issues introduced by the changes.\"\n", sessionID)
	b.WriteString("  3. If Codex finds new/reopened issues → verify and present them, let user decide again.\n")
	b.WriteString("  4. Max 5 verify rounds.\n")
	b.WriteString("- When all work is done → invoke write-report skill with session ID (write-report handles cleanup automatically).\n")
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
