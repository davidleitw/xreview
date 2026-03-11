package parser

import (
	"strings"
	"testing"
)

func TestExtractJSON_CleanJSON(t *testing.T) {
	input := `{"verdict":"APPROVED","summary":"clean","findings":[]}`
	result, err := ExtractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("expected input unchanged, got %q", result)
	}
}

func TestExtractJSON_CodeFences(t *testing.T) {
	input := "```json\n{\"verdict\":\"APPROVED\"}\n```"
	result, err := ExtractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"verdict":"APPROVED"}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExtractJSON_Empty(t *testing.T) {
	_, err := ExtractJSON("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	_, err := ExtractJSON("some random text without json")
	if err == nil {
		t.Fatal("expected error for non-JSON input")
	}
}

func TestExtractJSON_EmbeddedInText(t *testing.T) {
	input := "Here is my review:\n{\"verdict\":\"APPROVED\",\"summary\":\"looks good\",\"findings\":[]}\nDone."
	result, err := ExtractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result, "{") {
		t.Errorf("expected JSON starting with '{', got %q", result[:20])
	}
}

func TestExtractJSON_EmbeddedInTextWithNestedBraces(t *testing.T) {
	input := "Review:\n{\"verdict\":\"REVISE\",\"summary\":\"issues\",\"findings\":[{\"id\":\"F-001\"}]}\nEnd."
	result, err := ExtractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(result, "{") {
		t.Errorf("expected JSON starting with '{', got %q", result[:20])
	}
}

func TestExtractCodexSessionID_Found(t *testing.T) {
	stderr := "Session started: a1b2c3d4-e5f6-7890-abcd-ef1234567890\nSome other output"
	id, err := ExtractCodexSessionID(stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("unexpected session ID: %s", id)
	}
}

func TestExtractCodexSessionID_NotFound(t *testing.T) {
	_, err := ExtractCodexSessionID("no uuid here")
	if err == nil {
		t.Fatal("expected error when no UUID found")
	}
}

func TestParse_ValidJSON(t *testing.T) {
	input := `{"verdict":"NEEDS_REVIEW","summary":"found issues","findings":[{"id":"F001","severity":"high","category":"security","file":"main.go","line":42,"description":"SQL injection","suggestion":"use params"}]}`
	p := NewParser()
	resp, err := p.Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Verdict != "NEEDS_REVIEW" {
		t.Errorf("expected NEEDS_REVIEW, got %s", resp.Verdict)
	}
	if len(resp.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(resp.Findings))
	}
	if resp.Findings[0].ID != "F001" {
		t.Errorf("expected F001, got %s", resp.Findings[0].ID)
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	p := NewParser()
	_, err := p.Parse("not json at all")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParse_WithCodeFences(t *testing.T) {
	input := "```json\n{\"verdict\":\"APPROVED\",\"summary\":\"clean\",\"findings\":[]}\n```"
	p := NewParser()
	resp, err := p.Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Verdict != "APPROVED" {
		t.Errorf("expected APPROVED, got %s", resp.Verdict)
	}
}
