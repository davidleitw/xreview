# Plan 7: Formatter — XML Output Generation

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Generate structured XML output on stdout for all xreview commands. This is the primary interface between xreview and Claude Code skill.

**Architecture:** `internal/formatter/` package. `xml.go` for success output (review, verify, preflight, version, report, clean). `error.go` for error XML. Uses Go `encoding/xml` with custom struct tags.

**Tech Stack:** Go stdlib (`encoding/xml`, `fmt`, `io`)

**Depends on:** Plan 1 (types)

---

## Chunk 1: XML Output + Error Formatting

### File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `internal/formatter/xml.go` | XML structs + render functions for all command outputs |
| Create | `internal/formatter/xml_test.go` | XML output tests |
| Create | `internal/formatter/error.go` | Error XML generation with error codes |
| Create | `internal/formatter/error_test.go` | Error output tests |

---

### Task 7.1: XML Output Structs + Rendering

**Files:**
- Create: `internal/formatter/xml.go`
- Test: `internal/formatter/xml_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/formatter/xml_test.go`:

```go
package formatter

import (
	"strings"
	"testing"

	"github.com/davidleitw/xreview/internal/session"
)

func TestRenderReview(t *testing.T) {
	findings := []session.Finding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			Status:      "open",
			File:        "src/auth.go",
			Line:        42,
			Description: "JWT not checked",
			Suggestion:  "Add validation",
			CodeSnippet: "token, _ := jwt.Parse(raw, kf)",
		},
	}
	summary := session.FindingSummary{Total: 1, Open: 1}

	xml, err := RenderReview("xr-20260310-a1b2c3", 1, findings, summary, false)
	if err != nil {
		t.Fatalf("RenderReview: %v", err)
	}

	checks := []string{
		`status="success"`,
		`action="review"`,
		`session="xr-20260310-a1b2c3"`,
		`round="1"`,
		`id="F001"`,
		`severity="high"`,
		`file="src/auth.go"`,
		`line="42"`,
		`<description>JWT not checked</description>`,
		`<suggestion>Add validation</suggestion>`,
		`total="1"`,
		`open="1"`,
	}

	for _, c := range checks {
		if !strings.Contains(xml, c) {
			t.Errorf("missing: %s\nin:\n%s", c, xml)
		}
	}
}

func TestRenderVerify(t *testing.T) {
	findings := []session.Finding{
		{
			ID:               "F001",
			Severity:         "high",
			Category:         "security",
			Status:           "fixed",
			File:             "src/auth.go",
			Line:             42,
			Description:      "JWT not checked",
			VerificationNote: "Fix confirmed",
		},
	}
	summary := session.FindingSummary{Total: 1, Fixed: 1}

	xml, err := RenderReview("xr-20260310-a1b2c3", 2, findings, summary, false)
	if err != nil {
		t.Fatalf("RenderVerify: %v", err)
	}

	if !strings.Contains(xml, `status="fixed"`) {
		t.Error("missing fixed status")
	}
	if !strings.Contains(xml, `<verification>Fix confirmed</verification>`) {
		t.Error("missing verification note")
	}
}

func TestRenderPreflight_Success(t *testing.T) {
	checks := []PreflightCheck{
		{Name: "codex_installed", Passed: true, Detail: "found at /usr/bin/codex"},
		{Name: "codex_authenticated", Passed: true, Detail: "user@example.com"},
		{Name: "codex_responsive", Passed: true, Detail: "responded in 1.2s"},
	}

	xml, err := RenderPreflight(checks)
	if err != nil {
		t.Fatalf("RenderPreflight: %v", err)
	}

	if !strings.Contains(xml, `status="success"`) {
		t.Error("missing success status")
	}
	if !strings.Contains(xml, `name="codex_installed"`) {
		t.Error("missing check name")
	}
}

func TestRenderVersion(t *testing.T) {
	xml, err := RenderVersion("0.1.0", "0.2.0", true)
	if err != nil {
		t.Fatalf("RenderVersion: %v", err)
	}

	if !strings.Contains(xml, `current="0.1.0"`) {
		t.Error("missing current version")
	}
	if !strings.Contains(xml, `outdated="true"`) {
		t.Error("missing outdated flag")
	}
}

func TestRenderReport(t *testing.T) {
	xml, err := RenderReport("xr-20260310-a1b2c3", ".xreview/sessions/xr-20260310-a1b2c3/report.md", 3, session.FindingSummary{Total: 5, Open: 1, Fixed: 3, Dismissed: 1})
	if err != nil {
		t.Fatalf("RenderReport: %v", err)
	}

	if !strings.Contains(xml, `action="report"`) {
		t.Error("missing action")
	}
	if !strings.Contains(xml, `rounds="3"`) {
		t.Error("missing rounds")
	}
}

func TestRenderClean(t *testing.T) {
	xml, err := RenderClean("xr-20260310-a1b2c3")
	if err != nil {
		t.Fatalf("RenderClean: %v", err)
	}

	if !strings.Contains(xml, `action="clean"`) {
		t.Error("missing action")
	}
}

func TestRenderFullRescan(t *testing.T) {
	findings := []session.Finding{
		{ID: "F003", Severity: "low", Status: "open", Comparison: "recurring", File: "utils.go", Line: 88, Description: "nil ptr"},
		{ID: "F005", Severity: "high", Status: "open", Comparison: "new", File: "auth.go", Line: 60, Description: "hardcoded secret"},
	}
	resolved := []session.ResolvedFinding{
		{ID: "F001", Note: "no longer detected"},
	}
	summary := session.FindingSummary{Total: 2, Open: 2}

	xml, err := RenderFullRescan("xr-20260310-a1b2c3", 3, findings, resolved, summary)
	if err != nil {
		t.Fatalf("RenderFullRescan: %v", err)
	}

	if !strings.Contains(xml, `full-rescan="true"`) {
		t.Error("missing full-rescan attribute")
	}
	if !strings.Contains(xml, `comparison="recurring"`) {
		t.Error("missing recurring comparison")
	}
	if !strings.Contains(xml, `comparison="new"`) {
		t.Error("missing new comparison")
	}
	if !strings.Contains(xml, `<resolved`) {
		t.Error("missing resolved section")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -v`
Expected: FAIL — package not defined

- [ ] **Step 3: Write xml.go**

Create `internal/formatter/xml.go`:

```go
package formatter

import (
	"fmt"
	"strings"

	"github.com/davidleitw/xreview/internal/session"
)

// PreflightCheck represents a single preflight check result.
type PreflightCheck struct {
	Name   string
	Passed bool
	Detail string
}

// RenderReview generates XML for review/verify results.
func RenderReview(sessionID string, round int, findings []session.Finding, summary session.FindingSummary, fullRescan bool) (string, error) {
	action := "review"
	if round > 1 && !fullRescan {
		action = "verify"
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<xreview-result status="success" action="%s" session="%s" round="%d">`, action, sessionID, round)
	b.WriteString("\n")

	for _, f := range findings {
		renderFinding(&b, f)
	}

	fmt.Fprintf(&b, `  <summary total="%d" open="%d" fixed="%d" dismissed="%d" />`, summary.Total, summary.Open, summary.Fixed, summary.Dismissed)
	b.WriteString("\n")
	b.WriteString("</xreview-result>\n")

	return b.String(), nil
}

// RenderFullRescan generates XML for full-rescan results with comparison metadata.
func RenderFullRescan(sessionID string, round int, findings []session.Finding, resolved []session.ResolvedFinding, summary session.FindingSummary) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, `<xreview-result status="success" action="review" session="%s" round="%d" full-rescan="true">`, sessionID, round)
	b.WriteString("\n")

	for _, f := range findings {
		renderFindingWithComparison(&b, f)
	}

	if len(resolved) > 0 {
		b.WriteString("  <resolved-from-previous>\n")
		for _, r := range resolved {
			fmt.Fprintf(&b, `    <resolved id="%s" note="%s" />`, xmlEscape(r.ID), xmlEscape(r.Note))
			b.WriteString("\n")
		}
		b.WriteString("  </resolved-from-previous>\n")
	}

	fmt.Fprintf(&b, `  <summary total="%d" open="%d" fixed="%d" dismissed="%d" />`, summary.Total, summary.Open, summary.Fixed, summary.Dismissed)
	b.WriteString("\n")
	b.WriteString("</xreview-result>\n")

	return b.String(), nil
}

// RenderPreflight generates XML for preflight check results.
func RenderPreflight(checks []PreflightCheck) (string, error) {
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
	fmt.Fprintf(&b, `<xreview-result status="%s" action="preflight">`, status)
	b.WriteString("\n")

	b.WriteString("  <checks>\n")
	for _, c := range checks {
		fmt.Fprintf(&b, `    <check name="%s" passed="%t"`, c.Name, c.Passed)
		if c.Detail != "" {
			fmt.Fprintf(&b, ` detail="%s"`, xmlEscape(c.Detail))
		}
		b.WriteString(" />\n")
	}
	b.WriteString("  </checks>\n")

	b.WriteString("</xreview-result>\n")
	return b.String(), nil
}

// RenderVersion generates XML for version command output.
func RenderVersion(current, latest string, outdated bool) (string, error) {
	var b strings.Builder
	b.WriteString(`<xreview-result status="success" action="version">`)
	b.WriteString("\n")

	fmt.Fprintf(&b, `  <version current="%s" latest="%s" outdated="%t"`, current, latest, outdated)
	if outdated {
		b.WriteString(` update-command="go install github.com/davidleitw/xreview@latest"`)
	}
	b.WriteString(" />\n")

	b.WriteString("</xreview-result>\n")
	return b.String(), nil
}

// RenderReport generates XML for report command output.
func RenderReport(sessionID, reportPath string, rounds int, summary session.FindingSummary) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, `<xreview-result status="success" action="report" session="%s">`, sessionID)
	b.WriteString("\n")
	fmt.Fprintf(&b, `  <report path="%s" />`, reportPath)
	b.WriteString("\n")
	fmt.Fprintf(&b, `  <summary rounds="%d" total-findings="%d" fixed="%d" dismissed="%d" open="%d" />`,
		rounds, summary.Total, summary.Fixed, summary.Dismissed, summary.Open)
	b.WriteString("\n")
	b.WriteString("</xreview-result>\n")
	return b.String(), nil
}

// RenderClean generates XML for clean command output.
func RenderClean(sessionID string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, `<xreview-result status="success" action="clean" session="%s">`, sessionID)
	b.WriteString("\n")
	fmt.Fprintf(&b, `  <cleaned session="%s" />`, sessionID)
	b.WriteString("\n")
	b.WriteString("</xreview-result>\n")
	return b.String(), nil
}

// RenderSelfUpdate generates XML for self-update command output.
func RenderSelfUpdate(from, to string, alreadyLatest bool) (string, error) {
	var b strings.Builder
	b.WriteString(`<xreview-result status="success" action="self-update">`)
	b.WriteString("\n")
	fmt.Fprintf(&b, `  <update from="%s" to="%s"`, from, to)
	if alreadyLatest {
		b.WriteString(` already-latest="true"`)
	}
	b.WriteString(" />\n")
	b.WriteString("</xreview-result>\n")
	return b.String(), nil
}

func renderFinding(b *strings.Builder, f session.Finding) {
	fmt.Fprintf(b, `  <finding id="%s" severity="%s" category="%s" status="%s">`,
		xmlEscape(f.ID), xmlEscape(f.Severity), xmlEscape(f.Category), xmlEscape(f.Status))
	b.WriteString("\n")
	fmt.Fprintf(b, `    <location file="%s" line="%d" />`, xmlEscape(f.File), f.Line)
	b.WriteString("\n")
	fmt.Fprintf(b, "    <description>%s</description>", xmlEscape(f.Description))
	b.WriteString("\n")
	if f.Suggestion != "" {
		fmt.Fprintf(b, "    <suggestion>%s</suggestion>", xmlEscape(f.Suggestion))
		b.WriteString("\n")
	}
	if f.CodeSnippet != "" {
		fmt.Fprintf(b, "    <code-snippet>%s</code-snippet>", xmlEscape(f.CodeSnippet))
		b.WriteString("\n")
	}
	if f.VerificationNote != "" {
		fmt.Fprintf(b, "    <verification>%s</verification>", xmlEscape(f.VerificationNote))
		b.WriteString("\n")
	}
	b.WriteString("  </finding>\n")
}

func renderFindingWithComparison(b *strings.Builder, f session.Finding) {
	fmt.Fprintf(b, `  <finding id="%s" severity="%s" category="%s" status="%s" comparison="%s">`,
		xmlEscape(f.ID), xmlEscape(f.Severity), xmlEscape(f.Category), xmlEscape(f.Status), xmlEscape(f.Comparison))
	b.WriteString("\n")
	fmt.Fprintf(b, `    <location file="%s" line="%d" />`, xmlEscape(f.File), f.Line)
	b.WriteString("\n")
	fmt.Fprintf(b, "    <description>%s</description>", xmlEscape(f.Description))
	b.WriteString("\n")
	if f.Suggestion != "" {
		fmt.Fprintf(b, "    <suggestion>%s</suggestion>", xmlEscape(f.Suggestion))
		b.WriteString("\n")
	}
	if f.CodeSnippet != "" {
		fmt.Fprintf(b, "    <code-snippet>%s</code-snippet>", xmlEscape(f.CodeSnippet))
		b.WriteString("\n")
	}
	b.WriteString("  </finding>\n")
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/formatter/xml.go internal/formatter/xml_test.go
git commit -m "feat: add XML output formatter for all command types"
```

---

### Task 7.2: Error XML Generation

**Files:**
- Create: `internal/formatter/error.go`
- Test: `internal/formatter/error_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/formatter/error_test.go`:

```go
package formatter

import (
	"strings"
	"testing"
)

func TestRenderError(t *testing.T) {
	tests := []struct {
		code    string
		action  string
		message string
	}{
		{"CODEX_NOT_FOUND", "preflight", "codex CLI is not found in PATH"},
		{"CODEX_TIMEOUT", "review", "codex did not respond within 180 seconds"},
		{"SESSION_NOT_FOUND", "review", "Session 'xr-123' not found"},
		{"PARSE_FAILURE", "review", "Could not parse codex output"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			xml, err := RenderError(tt.action, tt.code, tt.message)
			if err != nil {
				t.Fatalf("RenderError: %v", err)
			}
			if !strings.Contains(xml, `status="error"`) {
				t.Error("missing error status")
			}
			if !strings.Contains(xml, `code="`+tt.code+`"`) {
				t.Error("missing error code")
			}
			if !strings.Contains(xml, tt.message) {
				t.Error("missing error message")
			}
		})
	}
}

func TestRenderPreflightError(t *testing.T) {
	checks := []PreflightCheck{
		{Name: "codex_installed", Passed: true, Detail: "found"},
		{Name: "codex_authenticated", Passed: false},
	}

	xml, err := RenderPreflightError("CODEX_NOT_AUTHENTICATED", "codex not logged in", checks)
	if err != nil {
		t.Fatalf("RenderPreflightError: %v", err)
	}

	if !strings.Contains(xml, `status="error"`) {
		t.Error("missing error status")
	}
	if !strings.Contains(xml, "CODEX_NOT_AUTHENTICATED") {
		t.Error("missing error code")
	}
	if !strings.Contains(xml, `name="codex_installed"`) {
		t.Error("missing passing check")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -v -run "TestRenderError|TestRenderPreflightError"`
Expected: FAIL — functions not defined

- [ ] **Step 3: Write error.go**

Create `internal/formatter/error.go`:

```go
package formatter

import (
	"fmt"
	"strings"
)

// RenderError generates error XML for any command.
func RenderError(action, code, message string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, `<xreview-result status="error" action="%s">`, action)
	b.WriteString("\n")
	fmt.Fprintf(&b, `  <error code="%s">`, code)
	b.WriteString("\n")
	fmt.Fprintf(&b, "    %s", xmlEscape(message))
	b.WriteString("\n")
	b.WriteString("  </error>\n")
	b.WriteString("</xreview-result>\n")
	return b.String(), nil
}

// RenderPreflightError generates error XML for preflight with partial check results.
func RenderPreflightError(code, message string, checks []PreflightCheck) (string, error) {
	var b strings.Builder
	b.WriteString(`<xreview-result status="error" action="preflight">`)
	b.WriteString("\n")
	fmt.Fprintf(&b, `  <error code="%s">`, code)
	b.WriteString("\n")
	fmt.Fprintf(&b, "    %s", xmlEscape(message))
	b.WriteString("\n")
	b.WriteString("  </error>\n")

	b.WriteString("  <checks>\n")
	for _, c := range checks {
		fmt.Fprintf(&b, `    <check name="%s" passed="%t"`, c.Name, c.Passed)
		if c.Detail != "" {
			fmt.Fprintf(&b, ` detail="%s"`, xmlEscape(c.Detail))
		}
		b.WriteString(" />\n")
	}
	b.WriteString("  </checks>\n")

	b.WriteString("</xreview-result>\n")
	return b.String(), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/formatter/error.go internal/formatter/error_test.go
git commit -m "feat: add error XML formatter with error codes"
```
