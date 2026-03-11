package formatter

import (
	"strings"
	"testing"

	"github.com/davidleitw/xreview/internal/session"
)

func TestFormatError(t *testing.T) {
	result := FormatError("review", ErrCodexTimeout, "codex did not respond within 180 seconds")

	assertContains(t, result, `status="error"`)
	assertContains(t, result, `action="review"`)
	assertContains(t, result, `code="CODEX_TIMEOUT"`)
	assertContains(t, result, "codex did not respond within 180 seconds")
}

func TestFormatError_XMLEscape(t *testing.T) {
	result := FormatError("review", "TEST", `message with "quotes" & <tags>`)

	assertContains(t, result, "&amp;")
	assertContains(t, result, "&lt;tags&gt;")
	assertContains(t, result, "&quot;quotes&quot;")
}

func TestFormatReviewResult_WithFindings(t *testing.T) {
	findings := []session.Finding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			Status:      "open",
			File:        "main.go",
			Line:        42,
			Description: "SQL injection vulnerability",
			Suggestion:  "Use parameterized queries",
			CodeSnippet: `db.Query("SELECT * FROM users WHERE id=" + id)`,
		},
	}
	summary := session.FindingSummary{Total: 1, Open: 1}

	result := FormatReviewResult("xr-20260310-abc123", 1, "NEEDS_REVIEW", findings, summary)

	assertContains(t, result, `status="success"`)
	assertContains(t, result, `action="review"`)
	assertContains(t, result, `session="xr-20260310-abc123"`)
	assertContains(t, result, `round="1"`)
	assertContains(t, result, `<verdict>NEEDS_REVIEW</verdict>`)
	assertContains(t, result, `id="F001"`)
	assertContains(t, result, `severity="high"`)
	assertContains(t, result, `category="security"`)
	assertContains(t, result, `file="main.go"`)
	assertContains(t, result, `line="42"`)
	assertContains(t, result, `<description>SQL injection vulnerability</description>`)
	assertContains(t, result, `<suggestion>Use parameterized queries</suggestion>`)
	assertContains(t, result, `<code-snippet>`)
	assertContains(t, result, `total="1"`)
	assertContains(t, result, `open="1"`)
}

func TestFormatReviewResult_NoFindings(t *testing.T) {
	summary := session.FindingSummary{Total: 0}

	result := FormatReviewResult("xr-20260310-abc123", 1, "APPROVED", nil, summary)

	assertContains(t, result, `<verdict>APPROVED</verdict>`)
	assertContains(t, result, `total="0"`)
	assertNotContains(t, result, `<finding`)
}

func TestFormatReviewResult_OptionalFields(t *testing.T) {
	findings := []session.Finding{
		{
			ID:          "F001",
			Severity:    "low",
			Category:    "style",
			Status:      "open",
			File:        "main.go",
			Line:        1,
			Description: "minor issue",
		},
	}
	summary := session.FindingSummary{Total: 1, Open: 1}

	result := FormatReviewResult("s1", 1, "NEEDS_REVIEW", findings, summary)

	// Should NOT contain optional elements when empty
	assertNotContains(t, result, `<suggestion>`)
	assertNotContains(t, result, `<code-snippet>`)
	assertNotContains(t, result, `<verification>`)
}

func TestFormatPreflightResult_AllPassed(t *testing.T) {
	checks := []Check{
		{Name: "codex_installed", Passed: true, Detail: "codex v1.0"},
		{Name: "codex_authenticated", Passed: true, Detail: "logged in"},
	}

	result := FormatPreflightResult(checks, "1.0.0", "1.0.0", false)

	assertContains(t, result, `status="success"`)
	assertContains(t, result, `action="preflight"`)
	assertContains(t, result, `name="codex_installed"`)
	assertContains(t, result, `passed="true"`)
	assertContains(t, result, `current="1.0.0"`)
	assertContains(t, result, `update-available="false"`)
}

func TestFormatPreflightResult_SomeFailed(t *testing.T) {
	checks := []Check{
		{Name: "codex_installed", Passed: true, Detail: "codex v1.0"},
		{Name: "codex_authenticated", Passed: false, Detail: "not logged in"},
	}

	result := FormatPreflightResult(checks, "1.0.0", "1.0.0", false)

	assertContains(t, result, `status="error"`)
}

func TestFormatPreflightResult_UpdateAvailable(t *testing.T) {
	checks := []Check{
		{Name: "codex_installed", Passed: true, Detail: "codex v1.0"},
	}

	result := FormatPreflightResult(checks, "0.1.0", "0.2.0", true)

	assertContains(t, result, `current="0.1.0"`)
	assertContains(t, result, `latest="0.2.0"`)
	assertContains(t, result, `update-available="true"`)
}

func TestFormatPreflightResult_NoLatestVersion(t *testing.T) {
	checks := []Check{
		{Name: "codex_installed", Passed: true, Detail: "codex v1.0"},
	}

	result := FormatPreflightResult(checks, "0.1.0", "", false)

	assertContains(t, result, `current="0.1.0"`)
	assertNotContains(t, result, `latest=`)
	assertNotContains(t, result, `update-available`)
}

func TestFormatSelfUpdateResult(t *testing.T) {
	result := FormatSelfUpdateResult("0.2.0")

	assertContains(t, result, `status="success"`)
	assertContains(t, result, `action="self-update"`)
	assertContains(t, result, `new="0.2.0"`)
}

func TestFormatVersionResult_Outdated(t *testing.T) {
	result := FormatVersionResult("0.1.0", "0.2.0", true)

	assertContains(t, result, `current="0.1.0"`)
	assertContains(t, result, `latest="0.2.0"`)
	assertContains(t, result, `outdated="true"`)
	assertContains(t, result, `update-command="xreview self-update"`)
}

func TestFormatVersionResult_UpToDate(t *testing.T) {
	result := FormatVersionResult("0.2.0", "0.2.0", false)

	assertContains(t, result, `outdated="false"`)
	assertContains(t, result, `update-command=""`)
}

func TestFormatReportResult(t *testing.T) {
	summary := session.FindingSummary{Total: 5, Open: 1, Fixed: 3, Dismissed: 1}
	result := FormatReportResult("xr-20260310-abc", "/tmp/report.md", summary)

	assertContains(t, result, `action="report"`)
	assertContains(t, result, `session="xr-20260310-abc"`)
	assertContains(t, result, `path="/tmp/report.md"`)
	assertContains(t, result, `total="5"`)
	assertContains(t, result, `fixed="3"`)
}

func TestFormatCleanResult(t *testing.T) {
	result := FormatCleanResult("xr-20260310-abc")

	assertContains(t, result, `action="clean"`)
	assertContains(t, result, `session="xr-20260310-abc"`)
	assertContains(t, result, "deleted successfully")
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, s)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected output NOT to contain %q, got:\n%s", substr, s)
	}
}
