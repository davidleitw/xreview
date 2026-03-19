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
		Context:     "【變更類型】feature【描述】add user auth",
		FetchMethod: "git diff HEAD~1..HEAD -- auth.go handler.go",
		FileList:    "auth.go\nhandler.go",
	}

	result, err := b.BuildFirstRound(input)
	if err != nil {
		t.Fatalf("BuildFirstRound failed: %v", err)
	}

	assertContains(t, result, "CRITICAL_RULES")
	assertContains(t, result, "add user auth")
	assertContains(t, result, "auth.go")
	assertContains(t, result, "git diff HEAD~1..HEAD")
	assertContains(t, result, "senior code reviewer")
}

func TestBuildFirstRound_InstructionOnly(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := FirstRoundInput{
		Context:     "【變更類型】feature【描述】add auth",
		FetchMethod: "git diff HEAD~3..HEAD",
		FileList:    "auth.go (45 lines)\nhandler.go (120 lines)",
	}

	result, err := b.BuildFirstRound(input)
	if err != nil {
		t.Fatalf("BuildFirstRound failed: %v", err)
	}

	assertContains(t, result, "git diff HEAD~3..HEAD")
	assertContains(t, result, "auth.go (45 lines)")
	assertNotContains(t, result, "===== DIFF =====")
	assertContains(t, result, "CRITICAL_RULES")
	assertContains(t, result, "trigger")
}

func TestBuildResume(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := ResumeInput{
		Message:          "Fixed F001, dismissed F002 because it's a false positive",
		PreviousFindings: "[F001] (high/security) main.go:42 — SQL injection",
		FetchMethod:      "git diff HEAD~1..HEAD -- main.go",
		FileList:         "main.go (10 lines)",
	}

	result, err := b.BuildResume(input)
	if err != nil {
		t.Fatalf("BuildResume failed: %v", err)
	}

	assertContains(t, result, "follow-up review")
	assertContains(t, result, "Fixed F001")
	assertContains(t, result, "SQL injection")
	assertContains(t, result, "git diff HEAD~1..HEAD -- main.go")
	assertNotContains(t, result, "===== UPDATED FILES =====")
}

func TestBuildResume_InstructionOnly(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := ResumeInput{
		Message:          "Fixed F001 with parameterized query",
		PreviousFindings: "[F001] (high/security) db.go:19 — SQL injection [status: open]",
		FetchMethod:      "git diff HEAD~1..HEAD -- db.go",
		FileList:         "db.go (25 lines)",
	}

	result, err := b.BuildResume(input)
	if err != nil {
		t.Fatalf("BuildResume failed: %v", err)
	}

	assertContains(t, result, "git diff HEAD~1..HEAD -- db.go")
	assertContains(t, result, "Fixed F001")
	assertContains(t, result, "SQL injection")
	assertNotContains(t, result, "===== UPDATED FILES =====")
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
	assertContains(t, result, "high/security")
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
		FetchMethod:      "git diff HEAD~1..HEAD",
		FileList:         "main.go (10 lines)",
	}

	result, err := b.BuildResume(input)
	if err != nil {
		t.Fatalf("BuildResume failed: %v", err)
	}

	assertContains(t, result, "\"verdict\"")
	assertContains(t, result, "\"findings\"")
	assertContains(t, result, "JSON")
}

func TestBuildFirstRound_ContainsEnrichedFieldGuidance(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := FirstRoundInput{
		Context:     "test",
		FetchMethod: "git diff HEAD",
		FileList:    "main.go",
	}

	result, err := b.BuildFirstRound(input)
	if err != nil {
		t.Fatalf("BuildFirstRound failed: %v", err)
	}

	assertContains(t, result, "trigger")
	assertContains(t, result, "cascade_impact")
	assertContains(t, result, "fix_alternatives")
	assertContains(t, result, "recommended")
}

func TestBuildResume_ContainsEnrichedFields(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := ResumeInput{
		Message:          "Fixed F001",
		PreviousFindings: "[F001] (high/security) main.go:42",
		FetchMethod:      "git diff HEAD~1..HEAD",
		FileList:         "main.go (10 lines)",
	}

	result, err := b.BuildResume(input)
	if err != nil {
		t.Fatalf("BuildResume failed: %v", err)
	}

	assertContains(t, result, `"trigger"`)
	assertContains(t, result, `"cascade_impact"`)
	assertContains(t, result, `"fix_alternatives"`)
}

func TestFormatFindingsForPrompt_EnrichedFields(t *testing.T) {
	b, _ := NewBuilder()

	findings := []session.Finding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			Status:      "open",
			File:        "db.go",
			Line:        19,
			Description: "SQL injection",
			Trigger:     "attacker sends malicious id",
			CascadeImpact: []string{
				"handler/task.go:GetTaskHandler() — passes input directly",
			},
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "Parameterized query", Effort: "minimal", Recommended: true},
			},
		},
	}

	result := b.FormatFindingsForPrompt(findings)

	assertContains(t, result, "Trigger: attacker sends malicious id")
	assertContains(t, result, "Cascade: handler/task.go:GetTaskHandler()")
	assertContains(t, result, "Fix A (recommended): Parameterized query [minimal]")
}

func TestBuildResume_ContainsScopedVerification(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := ResumeInput{
		Message:          "Fixed F001",
		PreviousFindings: "[F001] (high/security) main.go:42",
		FetchMethod:      "git diff HEAD~1..HEAD",
		FileList:         "main.go (10 lines)",
	}

	result, err := b.BuildResume(input)
	if err != nil {
		t.Fatalf("BuildResume failed: %v", err)
	}

	assertContains(t, result, "directly caused by")
	assertContains(t, result, "Do NOT report pre-existing issues")
	assertNotContains(t, result, "did any of the changes introduce NEW issues")
}

func TestBuildFirstRound_ContainsCascadeScanRule(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := FirstRoundInput{
		Context:     "test",
		FetchMethod: "git diff HEAD",
		FileList:    "main.go",
	}

	result, err := b.BuildFirstRound(input)
	if err != nil {
		t.Fatalf("BuildFirstRound failed: %v", err)
	}

	assertContains(t, result, "same pattern exists in other functions")
	assertContains(t, result, "Report ALL instances")
}

func TestBuildFirstRound_ContainsTODOExclusionRule(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := FirstRoundInput{
		Context:     "test",
		FetchMethod: "git diff HEAD",
		FileList:    "main.go",
	}

	result, err := b.BuildFirstRound(input)
	if err != nil {
		t.Fatalf("BuildFirstRound failed: %v", err)
	}

	assertContains(t, result, "TODO")
	assertContains(t, result, "BUG")
	assertContains(t, result, "FIXME")
}

func TestFormatFindingsForPrompt_IncludesConfidenceAndStrategy(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}
	findings := []session.Finding{
		{
			ID: "F-001", Severity: "high", Category: "security",
			File: "main.go", Line: 42, Description: "SQL injection",
			Status: "open", Confidence: 90, FixStrategy: "auto",
		},
	}
	result := b.FormatFindingsForPrompt(findings)
	assertContains(t, result, "confidence:90")
	assertContains(t, result, "strategy:auto")
}

func TestBuildFirstRound_ContainsConfidenceInstructions(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}
	input := FirstRoundInput{
		Context:     "test",
		FetchMethod: "read files",
		FileList:    "main.go",
	}
	result, err := b.BuildFirstRound(input)
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, result, "confidence")
	assertContains(t, result, "fix_strategy")
	assertContains(t, result, `"auto"`)
	assertContains(t, result, `"ask"`)
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
