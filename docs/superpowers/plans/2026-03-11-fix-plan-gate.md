# Fix Plan Gate Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enrich Codex findings with trigger/cascade/alternatives and add a mandatory Fix Plan approval gate in the skill workflow.

**Architecture:** Three new fields (`trigger`, `cascade_impact`, `fix_alternatives`) flow through the full pipeline: JSON schema → Codex prompt → Go types → parser → merge → XML formatter → skill presentation. The skill adds Step 2.5 (Fix Plan Gate) between review and fix execution.

**Tech Stack:** Go 1.22+, text/template, JSON schema, XML output, Claude Code skill (Markdown)

---

## Chunk 1: Go Data Layer (Types + Schema)

### Task 1: Add `FixAlternative` struct and enrich `Finding` / `CodexFinding` types

**Files:**
- Modify: `internal/session/types.go`
- Test: `internal/session/manager_test.go`

- [ ] **Step 1: Write failing test — new fields survive Create/Load round-trip**

Add to `internal/session/manager_test.go`:

```go
func TestManager_RoundTrip_EnrichedFindings(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	sess, err := mgr.Create([]string{"main.go"}, "files", "ctx", cfg)
	if err != nil {
		t.Fatal(err)
	}

	sess.Status = StatusInReview
	sess.Findings = []Finding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			Status:      FindingOpen,
			File:        "db.go",
			Line:        19,
			Description: "SQL injection",
			Suggestion:  "Use parameterized query",
			Trigger:     "attacker sends id=' OR '1'='1",
			CascadeImpact: []string{
				"handler/task.go:GetTaskHandler() — passes user input directly",
				"cache/task.go:GetCached() — bypasses DB validation",
			},
			FixAlternatives: []FixAlternative{
				{Label: "A", Description: "Parameterized query", Effort: "minimal", Recommended: true},
				{Label: "B", Description: "Introduce ORM", Effort: "large", Recommended: false},
			},
		},
	}

	if err := mgr.Update(sess); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	loaded, err := mgr.Load(sess.SessionID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	f := loaded.Findings[0]
	if f.Trigger != "attacker sends id=' OR '1'='1" {
		t.Errorf("trigger mismatch: got %q", f.Trigger)
	}
	if len(f.CascadeImpact) != 2 {
		t.Fatalf("expected 2 cascade impacts, got %d", len(f.CascadeImpact))
	}
	if f.CascadeImpact[0] != "handler/task.go:GetTaskHandler() — passes user input directly" {
		t.Errorf("cascade[0] mismatch: got %q", f.CascadeImpact[0])
	}
	if len(f.FixAlternatives) != 2 {
		t.Fatalf("expected 2 alternatives, got %d", len(f.FixAlternatives))
	}
	if f.FixAlternatives[0].Label != "A" || !f.FixAlternatives[0].Recommended {
		t.Errorf("alternative[0] mismatch: %+v", f.FixAlternatives[0])
	}
	if f.FixAlternatives[1].Effort != "large" {
		t.Errorf("alternative[1] effort mismatch: got %q", f.FixAlternatives[1].Effort)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -run TestManager_RoundTrip_EnrichedFindings -v`
Expected: FAIL — `Trigger`, `CascadeImpact`, `FixAlternatives`, `FixAlternative` undefined.

- [ ] **Step 3: Add types to `internal/session/types.go`**

Add `FixAlternative` struct after `FindingSummary`:

```go
// FixAlternative represents one possible fix approach for a finding.
type FixAlternative struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Effort      string `json:"effort"` // minimal, moderate, large
	Recommended bool   `json:"recommended"`
}
```

Add three fields to `Finding` struct (after `VerificationNote`):

```go
	Trigger         string           `json:"trigger,omitempty"`
	CascadeImpact   []string         `json:"cascade_impact,omitempty"`
	FixAlternatives []FixAlternative `json:"fix_alternatives,omitempty"`
```

Add the same three fields to `CodexFinding` struct (after `VerificationNote`):

```go
	Trigger         string           `json:"trigger,omitempty"`
	CascadeImpact   []string         `json:"cascade_impact,omitempty"`
	FixAlternatives []FixAlternative `json:"fix_alternatives,omitempty"`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -run TestManager_RoundTrip_EnrichedFindings -v`
Expected: PASS

- [ ] **Step 5: Run all session tests to check no regressions**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v`
Expected: All PASS (existing tests don't set new fields, so omitempty keeps JSON compatible).

- [ ] **Step 6: Update JSON schema `internal/schema/review.json`**

Add the three new properties inside `findings.items.properties`:

```json
"trigger": {
  "type": "string",
  "description": "Concrete trigger condition: specific input, scenario, or call sequence that manifests this issue"
},
"cascade_impact": {
  "type": "array",
  "items": { "type": "string" },
  "description": "Other codebase locations affected if this finding is fixed. Each entry: file:function — impact. Empty array if none."
},
"fix_alternatives": {
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "label": { "type": "string" },
      "description": { "type": "string" },
      "effort": { "type": "string", "enum": ["minimal", "moderate", "large"] },
      "recommended": { "type": "boolean" }
    },
    "required": ["label", "description", "effort", "recommended"],
    "additionalProperties": false
  },
  "description": "2-3 fix approaches with effort estimate. At least one must be recommended."
}
```

Add `"trigger"`, `"cascade_impact"`, `"fix_alternatives"` to the `required` array of the finding item.

- [ ] **Step 7: Verify schema is valid JSON**

Run: `cd /home/davidleitw/xreview && python3 -c "import json; json.load(open('internal/schema/review.json'))"`
Expected: No error.

- [ ] **Step 8: Commit**

```bash
git add internal/session/types.go internal/session/manager_test.go internal/schema/review.json
git commit -m "feat: add trigger, cascade_impact, fix_alternatives to Finding types and schema"
```

---

## Chunk 2: Prompt Templates

### Task 2: Update `FirstRoundTemplate` with new field guidance

**Files:**
- Modify: `internal/prompt/templates.go`
- Test: `internal/prompt/builder_test.go`

- [ ] **Step 1: Write failing test — first-round prompt contains new field guidance**

Add to `internal/prompt/builder_test.go`:

```go
func TestBuildFirstRound_ContainsEnrichedFieldGuidance(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	input := FirstRoundInput{
		Context:  "test",
		FileList: "main.go",
		Diff:     "+line",
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestBuildFirstRound_ContainsEnrichedFieldGuidance -v`
Expected: FAIL — template does not contain "trigger", "cascade_impact", etc.

- [ ] **Step 3: Update `FirstRoundTemplate` in `internal/prompt/templates.go`**

Append the following at the end of the template, after `===== END =====`:

```
For each finding, you MUST also provide these fields:
- trigger: the concrete input, scenario, or call sequence that manifests the issue.
  Be specific (e.g. "user sends id=' OR '1'='1") not abstract (e.g. "malicious input").
- cascade_impact: other files/functions in the repository that would be affected if
  this code is changed. Trace the call chain. Use format "file:function() — description".
  You are encouraged to read additional files to identify these. Empty array [] if none.
- fix_alternatives: provide 2-3 fix approaches. Each has label (A/B/C), description,
  effort (minimal/moderate/large), and recommended (true for exactly one).
  Consider trade-offs: minimal fix vs. systemic improvement.
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestBuildFirstRound_ContainsEnrichedFieldGuidance -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/prompt/templates.go internal/prompt/builder_test.go
git commit -m "feat: add enriched field guidance to FirstRoundTemplate"
```

### Task 3: Update `ResumeTemplate` with new fields in inline schema

**Files:**
- Modify: `internal/prompt/templates.go`
- Test: `internal/prompt/builder_test.go`

- [ ] **Step 1: Write failing test — resume prompt inline schema contains new fields**

Add to `internal/prompt/builder_test.go`:

```go
func TestBuildResume_ContainsEnrichedFields(t *testing.T) {
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

	assertContains(t, result, `"trigger"`)
	assertContains(t, result, `"cascade_impact"`)
	assertContains(t, result, `"fix_alternatives"`)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestBuildResume_ContainsEnrichedFields -v`
Expected: FAIL

- [ ] **Step 3: Update `ResumeTemplate` inline schema**

In the inline JSON schema at the bottom of `ResumeTemplate`, add the three new fields to the finding object example:

```
      "trigger": "concrete trigger condition",
      "cascade_impact": ["file:func() — impact description"],
      "fix_alternatives": [
        {"label": "A", "description": "fix approach", "effort": "minimal|moderate|large", "recommended": true}
      ]
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestBuildResume_ContainsEnrichedFields -v`
Expected: PASS

- [ ] **Step 5: Update `FormatFindingsForPrompt` to include new fields**

Add to `internal/prompt/builder_test.go`:

```go
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
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestFormatFindingsForPrompt_EnrichedFields -v`
Expected: FAIL

- [ ] **Step 7: Implement in `internal/prompt/builder.go`**

In `FormatFindingsForPrompt`, after the existing `VerificationNote` block, add:

```go
		if f.Trigger != "" {
			fmt.Fprintf(&buf, "  Trigger: %s\n", f.Trigger)
		}
		if len(f.CascadeImpact) > 0 {
			for _, ci := range f.CascadeImpact {
				fmt.Fprintf(&buf, "  Cascade: %s\n", ci)
			}
		}
		for _, alt := range f.FixAlternatives {
			rec := ""
			if alt.Recommended {
				rec = " (recommended)"
			}
			fmt.Fprintf(&buf, "  Fix %s%s: %s [%s]\n", alt.Label, rec, alt.Description, alt.Effort)
		}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestFormatFindingsForPrompt_EnrichedFields -v`
Expected: PASS

- [ ] **Step 9: Run all prompt tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -v`
Expected: All PASS

- [ ] **Step 10: Commit**

```bash
git add internal/prompt/templates.go internal/prompt/builder.go internal/prompt/builder_test.go
git commit -m "feat: add enriched fields to ResumeTemplate and FormatFindingsForPrompt"
```

---

## Chunk 3: XML Formatter

### Task 4: Add new XML elements for enriched finding fields

**Files:**
- Modify: `internal/formatter/xml.go`
- Test: `internal/formatter/formatter_test.go`

- [ ] **Step 1: Write failing test — XML output contains new elements**

Add to `internal/formatter/formatter_test.go`:

```go
func TestFormatReviewResult_EnrichedFields(t *testing.T) {
	findings := []session.Finding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			Status:      "open",
			File:        "db.go",
			Line:        19,
			Description: "SQL injection",
			Suggestion:  "Use parameterized query",
			Trigger:     "attacker sends id=' OR '1'='1",
			CascadeImpact: []string{
				"handler/task.go:GetTaskHandler() — passes input directly",
				"cache/task.go:GetCached() — bypasses validation",
			},
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "Parameterized query", Effort: "minimal", Recommended: true},
				{Label: "B", Description: "Introduce ORM", Effort: "large", Recommended: false},
			},
		},
	}
	summary := session.FindingSummary{Total: 1, Open: 1}

	result := FormatReviewResult("xr-test", 1, "REVISE", findings, summary)

	assertContains(t, result, `<trigger>attacker sends id=&#39; OR &#39;1&#39;=&#39;1</trigger>`)
	assertContains(t, result, `<cascade-impact>`)
	assertContains(t, result, `<impact>handler/task.go:GetTaskHandler()`)
	assertContains(t, result, `<impact>cache/task.go:GetCached()`)
	assertContains(t, result, `</cascade-impact>`)
	assertContains(t, result, `<fix-alternatives>`)
	assertContains(t, result, `<alternative label="A" effort="minimal" recommended="true">Parameterized query</alternative>`)
	assertContains(t, result, `<alternative label="B" effort="large" recommended="false">Introduce ORM</alternative>`)
	assertContains(t, result, `</fix-alternatives>`)
}
```

- [ ] **Step 2: Write test — empty enriched fields produce no XML elements**

Add to `internal/formatter/formatter_test.go`:

```go
func TestFormatReviewResult_NoEnrichedFields(t *testing.T) {
	findings := []session.Finding{
		{
			ID:          "F001",
			Severity:    "low",
			Category:    "logic",
			Status:      "open",
			File:        "main.go",
			Line:        1,
			Description: "minor issue",
		},
	}
	summary := session.FindingSummary{Total: 1, Open: 1}

	result := FormatReviewResult("xr-test", 1, "REVISE", findings, summary)

	assertNotContains(t, result, `<trigger>`)
	assertNotContains(t, result, `<cascade-impact>`)
	assertNotContains(t, result, `<fix-alternatives>`)
}
```

- [ ] **Step 2b: Write test — alternatives with no recommended renders without crash**

Add to `internal/formatter/formatter_test.go`:

```go
func TestFormatReviewResult_AlternativesNoRecommended(t *testing.T) {
	findings := []session.Finding{
		{
			ID:          "F001",
			Severity:    "medium",
			Category:    "logic",
			Status:      "open",
			File:        "main.go",
			Line:        10,
			Description: "potential issue",
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "option one", Effort: "minimal", Recommended: false},
				{Label: "B", Description: "option two", Effort: "moderate", Recommended: false},
			},
		},
	}
	summary := session.FindingSummary{Total: 1, Open: 1}

	result := FormatReviewResult("xr-test", 1, "REVISE", findings, summary)

	assertContains(t, result, `<fix-alternatives>`)
	assertContains(t, result, `recommended="false">option one</alternative>`)
	assertContains(t, result, `recommended="false">option two</alternative>`)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -run "TestFormatReviewResult_EnrichedFields|TestFormatReviewResult_NoEnrichedFields|TestFormatReviewResult_AlternativesNoRecommended" -v`
Expected: FAIL (first test fails because XML doesn't contain new elements; second may pass since elements aren't emitted yet).

- [ ] **Step 4: Implement in `internal/formatter/xml.go`**

In `FormatReviewResult`, after the existing `VerificationNote` block (`if f.VerificationNote != "" { ... }`) and before `b.WriteString("  </finding>\n")`, add:

```go
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
```

Also add `'` escaping to `xmlEscape` (for the test to pass with `&#39;`):

```go
	s = strings.ReplaceAll(s, "'", "&#39;")
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -run "TestFormatReviewResult_EnrichedFields|TestFormatReviewResult_NoEnrichedFields" -v`
Expected: PASS

- [ ] **Step 6: Run all formatter tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/formatter/xml.go internal/formatter/formatter_test.go
git commit -m "feat: add trigger, cascade-impact, fix-alternatives XML elements"
```

---

## Chunk 4: Reviewer (codexFindingsToFindings + mergeFindings)

### Task 5: Update `codexFindingsToFindings` to carry new fields

**Files:**
- Modify: `internal/reviewer/single.go`
- Test: `internal/reviewer/single_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/reviewer/single_test.go`:

```go
func TestCodexFindingsToFindings_EnrichedFields(t *testing.T) {
	cf := []session.CodexFinding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			File:        "db.go",
			Line:        19,
			Description: "SQL injection",
			Suggestion:  "Use parameterized query",
			Trigger:     "attacker sends malicious id",
			CascadeImpact: []string{
				"handler/task.go:GetTaskHandler() — passes input",
			},
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "Parameterized query", Effort: "minimal", Recommended: true},
			},
		},
	}

	result := codexFindingsToFindings(cf)

	if result[0].Trigger != "attacker sends malicious id" {
		t.Errorf("trigger mismatch: got %q", result[0].Trigger)
	}
	if len(result[0].CascadeImpact) != 1 {
		t.Fatalf("expected 1 cascade impact, got %d", len(result[0].CascadeImpact))
	}
	if len(result[0].FixAlternatives) != 1 {
		t.Fatalf("expected 1 fix alternative, got %d", len(result[0].FixAlternatives))
	}
	if result[0].FixAlternatives[0].Label != "A" {
		t.Errorf("alternative label mismatch: got %q", result[0].FixAlternatives[0].Label)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/reviewer/ -run TestCodexFindingsToFindings_EnrichedFields -v`
Expected: FAIL — new fields are zero-valued in result.

- [ ] **Step 3: Update `codexFindingsToFindings` in `internal/reviewer/single.go`**

Add the three new fields to the `Finding` literal inside the loop:

```go
		findings[i] = session.Finding{
			// ... existing fields ...
			Trigger:         f.Trigger,
			CascadeImpact:   f.CascadeImpact,
			FixAlternatives: f.FixAlternatives,
		}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/davidleitw/xreview && go test ./internal/reviewer/ -run TestCodexFindingsToFindings_EnrichedFields -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/reviewer/single.go internal/reviewer/single_test.go
git commit -m "feat: carry enriched fields through codexFindingsToFindings"
```

### Task 6: Update `mergeFindings` to handle new fields

**Files:**
- Modify: `internal/reviewer/single.go`
- Test: `internal/reviewer/single_test.go`

- [ ] **Step 1: Write failing test — merge updates enriched fields on existing findings**

Add to `internal/reviewer/single_test.go`:

```go
func TestMergeFindings_UpdateEnrichedFields(t *testing.T) {
	existing := []session.Finding{
		{
			ID:       "F001",
			Status:   "open",
			Trigger:  "old trigger",
			CascadeImpact: []string{"old cascade"},
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "old fix", Effort: "minimal", Recommended: true},
			},
		},
	}
	incoming := []session.CodexFinding{
		{
			ID:       "F001",
			Status:   "open",
			Trigger:  "updated trigger",
			CascadeImpact: []string{"new cascade 1", "new cascade 2"},
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "updated fix", Effort: "minimal", Recommended: true},
				{Label: "B", Description: "new option", Effort: "large", Recommended: false},
			},
		},
	}

	result := mergeFindings(existing, incoming)

	f := result[0]
	if f.Trigger != "updated trigger" {
		t.Errorf("trigger not updated: got %q", f.Trigger)
	}
	if len(f.CascadeImpact) != 2 {
		t.Fatalf("expected 2 cascade impacts, got %d", len(f.CascadeImpact))
	}
	if len(f.FixAlternatives) != 2 {
		t.Fatalf("expected 2 alternatives, got %d", len(f.FixAlternatives))
	}
}
```

- [ ] **Step 2: Write failing test — merge carries enriched fields on new findings**

Add to `internal/reviewer/single_test.go`:

```go
func TestMergeFindings_AddNewWithEnrichedFields(t *testing.T) {
	existing := []session.Finding{}
	incoming := []session.CodexFinding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			File:        "db.go",
			Line:        19,
			Description: "SQL injection",
			Trigger:     "malicious input",
			CascadeImpact: []string{"handler.go:Handle()"},
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "fix", Effort: "minimal", Recommended: true},
			},
		},
	}

	result := mergeFindings(existing, incoming)

	if len(result) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result))
	}
	if result[0].Trigger != "malicious input" {
		t.Errorf("trigger mismatch: got %q", result[0].Trigger)
	}
	if len(result[0].FixAlternatives) != 1 {
		t.Fatalf("expected 1 alternative, got %d", len(result[0].FixAlternatives))
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/reviewer/ -run "TestMergeFindings_UpdateEnrichedFields|TestMergeFindings_AddNewWithEnrichedFields" -v`
Expected: FAIL — enriched fields are zero-valued after merge.

- [ ] **Step 4: Update `mergeFindings` in `internal/reviewer/single.go`**

In the "update existing" branch (where `idx, ok := byID[cf.ID]`), add after the existing updates:

```go
			if cf.Trigger != "" {
				existing[idx].Trigger = cf.Trigger
			}
			if len(cf.CascadeImpact) > 0 {
				existing[idx].CascadeImpact = cf.CascadeImpact
			}
			if len(cf.FixAlternatives) > 0 {
				existing[idx].FixAlternatives = cf.FixAlternatives
			}
```

In the "new finding" branch (the `else` append), add the three fields:

```go
			existing = append(existing, session.Finding{
				// ... existing fields ...
				Trigger:         cf.Trigger,
				CascadeImpact:   cf.CascadeImpact,
				FixAlternatives: cf.FixAlternatives,
			})
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/reviewer/ -run "TestMergeFindings_UpdateEnrichedFields|TestMergeFindings_AddNewWithEnrichedFields" -v`
Expected: PASS

- [ ] **Step 6: Run all reviewer tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/reviewer/ -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/reviewer/single.go internal/reviewer/single_test.go
git commit -m "feat: handle enriched fields in mergeFindings"
```

---

## Chunk 5: Full Integration Test

### Task 7: End-to-end test — enriched fields flow through the full pipeline

**Files:**
- Test: `internal/reviewer/single_test.go`

- [ ] **Step 1: Write integration test — Review with enriched codex output**

Add to `internal/reviewer/single_test.go`:

```go
func TestReview_EnrichedFieldsEndToEnd(t *testing.T) {
	mgr := newMockManager()
	coll := &mockCollector{
		files: []collector.FileContent{
			{Path: "db.go", Content: "package db\nfunc GetTask(id string) {}\n", Lines: 2},
		},
	}
	runner := &mockRunner{
		execFn: func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
			return &codex.ExecResult{
				Stdout:         "{}",
				CodexSessionID: "codex-sess-456",
				DurationMs:     300,
			}, nil
		},
	}
	psr := &mockParser{
		parseFn: func(stdout string) (*session.CodexResponse, error) {
			return &session.CodexResponse{
				Verdict: "REVISE",
				Summary: "found security issue",
				Findings: []session.CodexFinding{
					{
						ID:          "F001",
						Severity:    "high",
						Category:    "security",
						File:        "db.go",
						Line:        2,
						Description: "SQL injection in GetTask",
						Suggestion:  "Use parameterized query",
						CodeSnippet: "func GetTask(id string) {}",
						Trigger:     "attacker sends id=' OR '1'='1",
						CascadeImpact: []string{
							"handler/task.go:GetTaskHandler() — passes user input directly",
						},
						FixAlternatives: []session.FixAlternative{
							{Label: "A", Description: "Parameterized query", Effort: "minimal", Recommended: true},
							{Label: "B", Description: "ORM layer", Effort: "large", Recommended: false},
						},
					},
				},
			}, nil
		},
	}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 180}

	r := NewSingleReviewer(runner, &mockBuilder{}, psr, mgr, coll, cfg)

	result, err := r.Review(context.Background(), ReviewRequest{
		Targets:    []string{"db.go"},
		TargetMode: "files",
		Context:    "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	f := result.Findings[0]
	if f.Trigger != "attacker sends id=' OR '1'='1" {
		t.Errorf("trigger not carried: got %q", f.Trigger)
	}
	if len(f.CascadeImpact) != 1 {
		t.Errorf("cascade not carried: got %d items", len(f.CascadeImpact))
	}
	if len(f.FixAlternatives) != 2 {
		t.Errorf("alternatives not carried: got %d items", len(f.FixAlternatives))
	}
	if !f.FixAlternatives[0].Recommended {
		t.Errorf("expected first alternative to be recommended")
	}

	// Verify session state also has enriched fields
	sess := mgr.sessions[result.SessionID]
	if sess.Findings[0].Trigger == "" {
		t.Error("session finding should have trigger")
	}
}
```

- [ ] **Step 2: Run test**

Run: `cd /home/davidleitw/xreview && go test ./internal/reviewer/ -run TestReview_EnrichedFieldsEndToEnd -v`
Expected: PASS (all changes from Tasks 1-6 make this work).

- [ ] **Step 3: Run the full test suite**

Run: `cd /home/davidleitw/xreview && go test ./... -v`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add internal/reviewer/single_test.go
git commit -m "test: add end-to-end test for enriched finding fields"
```

---

## Chunk 6: Skill Workflow + Reference Docs

### Task 8: Rewrite SKILL.md with Fix Plan Gate

**Files:**
- Modify: `.claude/skills/xreview/SKILL.md`

- [ ] **Step 1: Rewrite SKILL.md**

Keep Step 0 (Preflight), Step 1 (Determine targets), Step 2 (Run review) unchanged.

Replace Step 3 with **Step 2.5: Fix Plan Gate**:

```markdown
## Step 2.5: Fix Plan Gate (MANDATORY)

Parse the XML output from Step 2.

If verdict is APPROVED (zero findings): tell the user "No issues found." Skip to Step 5.

Otherwise, you MUST present ALL findings as a complete fix plan BEFORE touching any code.
This is a hard gate — do NOT start fixing anything until the user approves.

### Build the Fix Plan

For EACH finding, read the relevant source file around the finding location, then present:

**For high/security severity:**

\```
### F-001: SQL Injection (security/high)
📍 store/db.go:19 — GetTask()

**Trigger**: attacker sends id=' OR '1'='1 as taskID
**Root cause**: fmt.Sprintf concatenates user input directly into SQL
**Impact**: attacker can read, modify, or delete entire database
**Cascade**: if this code changes, also check:
  - handler/task.go:GetTaskHandler() — passes user input directly
  - cache/task.go:GetCached() — bypasses DB validation on cache miss

**Fix options**:
  A. (Recommended) Change to parameterized query — minimal effort
  B. Introduce ORM layer — large effort
  C. Don't fix — risk: full database compromise
\```

**For medium severity:**

Same structure, but cascade and alternatives may be shorter. Still include "Don't fix" option.

**For low severity:**

Brief description with recommended fix. Still include "Don't fix" option.

### Get User Approval

After listing ALL findings, use AskUserQuestion:

\```
Fix plan for N findings above. How to proceed?
  A. Execute all recommended fixes
  B. Only fix high severity, skip the rest
  C. I want to adjust (tell me which findings to change — e.g. "F-003 skip, F-005 use option B")
\```

Do NOT proceed until user responds.
```

Replace old Step 3 with **Step 3: Execute Fixes**:

```markdown
## Step 3: Execute Fixes

Execute fixes strictly per the approved plan. No re-analysis, no ad-hoc decisions.

For each finding marked for fix:
1. Apply the chosen fix approach.
2. Briefly report what you did (one line per finding).

If user chose option C with adjustments, follow those exactly.
Skip any finding the user chose to not fix.
```

Replace Step 4 with enhanced verification:

```markdown
## Step 4: Summary + Verify

Present a summary table:

\```
### Round N Summary

| ID    | Issue              | Action       | Detail                          |
|-------|--------------------|--------------|---------------------------------|
| F-001 | SQL injection      | Fixed (A)    | Changed to parameterized query  |
| F-002 | Unused error       | Not fixed    | User: acceptable for demo code  |
\```

Then run verification with enhanced scope:

`xreview review --session <session-id> --message "<message>"`

The message MUST include:
- Which findings were fixed and how
- Which findings were dismissed and why
- Explicit instruction: "Re-review ALL modified files. Beyond verifying old findings, also check:
  1. Whether fixes introduced new security/logic issues
  2. Unhandled cascade impact between fixes
  3. Cross-layer consistency (if DB layer changed, are cache/handler layers in sync)"

Parse the result:
- All resolved → proceed to Step 5.
- Codex disagrees or finds new issues → present new/reopened findings with the same Fix Plan format (Step 2.5), get user approval, then fix.
- Maximum 5 rounds → inform user of remaining items, proceed to Step 5.
```

Keep Step 5 (Finalize) unchanged.

- [ ] **Step 2: Review the rewritten SKILL.md for completeness**

Read the file back and verify:
- Step 0, 1, 2 unchanged
- Step 2.5 has mandatory gate with AskUserQuestion
- Step 3 follows approved plan only
- Step 4 has enhanced verification message
- Step 5 unchanged
- Important notes section still accurate

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/xreview/SKILL.md
git commit -m "feat: add Fix Plan Gate (Step 2.5) to skill workflow"
```

### Task 9: Update reference.md with new XML elements

**Files:**
- Modify: `.claude/skills/xreview/reference.md`

- [ ] **Step 1: Add new element documentation to `reference.md`**

Add after the `<finding>` section:

```markdown
### <trigger>
Content: Concrete trigger condition (specific input/scenario). Child of `<finding>`.

### <cascade-impact>
Children: `<impact>` elements. Each describes a codebase location affected by fixing this finding.

### <fix-alternatives>
Children: `<alternative>` elements.
`<alternative>` attributes: label (A/B/C), effort (minimal|moderate|large), recommended (true|false)
Content: description of the fix approach.
```

- [ ] **Step 2: Commit**

```bash
git add .claude/skills/xreview/reference.md
git commit -m "docs: add trigger, cascade-impact, fix-alternatives to XML reference"
```

### Task 10: Final full test suite run

- [ ] **Step 1: Run the complete test suite**

Run: `cd /home/davidleitw/xreview && go test ./... -v -count=1`
Expected: All PASS

- [ ] **Step 2: Build the binary**

Run: `cd /home/davidleitw/xreview && go build -o xreview ./cmd/xreview/`
Expected: Build succeeds with no errors.

- [ ] **Step 3: Smoke test preflight**

Run: `cd /home/davidleitw/xreview && ./xreview version`
Expected: Outputs version XML without errors.
