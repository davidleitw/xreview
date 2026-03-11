package prompt

import (
	"strings"
	"testing"

	"github.com/davidleitw/xreview/internal/session"
)

func TestNewBuilder(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil builder")
	}
}

func TestBuildFirstRound(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := FirstRoundInput{
		Context:  "【變更類型】feature【描述】add user auth",
		FileList: "auth.go\nhandler.go",
		Diff:     "+func Login() {}",
	}

	result, err := b.BuildFirstRound(input)
	if err != nil {
		t.Fatalf("BuildFirstRound failed: %v", err)
	}

	assertContains(t, result, "CRITICAL_RULES")
	assertContains(t, result, "add user auth")
	assertContains(t, result, "auth.go")
	assertContains(t, result, "+func Login() {}")
	assertContains(t, result, "senior code reviewer")
}

func TestBuildResume(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := ResumeInput{
		Message:          "Fixed F001, dismissed F002 because it's a false positive",
		PreviousFindings: "[F001] (high/security) main.go:42 — SQL injection",
		UpdatedFiles:     "package main\n// fixed",
	}

	result, err := b.BuildResume(input)
	if err != nil {
		t.Fatalf("BuildResume failed: %v", err)
	}

	assertContains(t, result, "follow-up review")
	assertContains(t, result, "Fixed F001")
	assertContains(t, result, "SQL injection")
	assertContains(t, result, "// fixed")
}

func TestFormatFindingsForPrompt_Empty(t *testing.T) {
	b, _ := NewBuilder()
	result := b.FormatFindingsForPrompt(nil)
	if result != "(no previous findings)" {
		t.Errorf("expected '(no previous findings)', got %q", result)
	}
}

func TestFormatFindingsForPrompt_WithFindings(t *testing.T) {
	b, _ := NewBuilder()

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
		},
		{
			ID:               "F002",
			Severity:         "medium",
			Category:         "logic",
			Status:           "fixed",
			File:             "handler.go",
			Line:             15,
			Description:      "Off-by-one error",
			VerificationNote: "Fixed by changing < to <=",
		},
	}

	result := b.FormatFindingsForPrompt(findings)

	assertContains(t, result, "[F001]")
	assertContains(t, result, "(high/security)")
	assertContains(t, result, "main.go:42")
	assertContains(t, result, "SQL injection vulnerability")
	assertContains(t, result, "[status: open]")
	assertContains(t, result, "Suggestion: Use parameterized queries")

	assertContains(t, result, "[F002]")
	assertContains(t, result, "[status: fixed]")
	assertContains(t, result, "Verification: Fixed by changing")
}

func TestFormatFindingsForPrompt_NoOptionalFields(t *testing.T) {
	b, _ := NewBuilder()

	findings := []session.Finding{
		{
			ID:          "F001",
			Severity:    "low",
			Category:    "style",
			Status:      "dismissed",
			File:        "main.go",
			Line:        1,
			Description: "minor issue",
		},
	}

	result := b.FormatFindingsForPrompt(findings)

	assertContains(t, result, "[F001]")
	assertNotContains(t, result, "Suggestion:")
	assertNotContains(t, result, "Verification:")
}

func TestBuildResume_ContainsJSONInstruction(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := ResumeInput{
		Message:          "Fixed F001",
		PreviousFindings: "[F001] (high/security) main.go:42",
		UpdatedFiles:     "package main",
	}

	result, err := b.BuildResume(input)
	if err != nil {
		t.Fatalf("BuildResume failed: %v", err)
	}

	assertContains(t, result, "\"verdict\"")
	assertContains(t, result, "\"findings\"")
	assertContains(t, result, "JSON")
}

func TestBuildResume_WithAdditionalFiles(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := ResumeInput{
		Message:          "Fixed F001, also review tests",
		PreviousFindings: "[F001] (high/security) main.go:42",
		UpdatedFiles:     "package main",
		AdditionalFiles:  "--- test_main.go ---\npackage main_test",
	}

	result, err := b.BuildResume(input)
	if err != nil {
		t.Fatalf("BuildResume failed: %v", err)
	}

	assertContains(t, result, "ADDITIONAL FILES")
	assertContains(t, result, "test_main.go")
}

func TestBuildResume_NoAdditionalFiles(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := ResumeInput{
		Message:          "Fixed F001",
		PreviousFindings: "[F001]",
		UpdatedFiles:     "package main",
		AdditionalFiles:  "",
	}

	result, err := b.BuildResume(input)
	if err != nil {
		t.Fatalf("BuildResume failed: %v", err)
	}

	assertNotContains(t, result, "ADDITIONAL FILES")
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected to contain %q, got:\n%s", substr, s)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected NOT to contain %q, got:\n%s", substr, s)
	}
}
