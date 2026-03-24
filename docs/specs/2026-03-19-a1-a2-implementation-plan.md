# A1 + A2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add confidence + fix_strategy fields to finding schema, simplify agent instructions to single-source-of-truth in SKILL.md, add session versioning, and update skill UX.

**Architecture:** Schema changes flow through: review.json (Codex schema) → types.go (Go structs) → templates.go (prompt) → xml.go (XML output + simplified agent instructions) → builder.go (resume format) → SKILL.md (workflow) → reference.md (docs). Session versioning rejects old sessions at Load() time.

**Tech Stack:** Go, JSON Schema, Go text/template, XML string builder

**Spec:** `docs/specs/2026-03-19-a1-a2-schema-and-skill-ux-design.md`

---

### Task 1: Add confidence + fix_strategy to Codex output schema

**Files:**
- Modify: `internal/schema/review.json`

- [ ] **Step 1: Add confidence property to review.json**

After line 51 (after `fix_alternatives` description closing brace), add:

```json
          "confidence": {
            "type": "integer",
            "minimum": 0,
            "maximum": 100,
            "description": "How certain you are this is a real issue (0=guess, 100=certain)"
          },
          "fix_strategy": {
            "type": "string",
            "enum": ["auto", "ask"],
            "description": "auto: senior engineer would fix without discussion. ask: needs human decision."
          }
```

- [ ] **Step 2: Add to required array**

Update line 53 required array to include the two new fields at the end:

```json
        "required": ["id", "severity", "category", "file", "line", "description", "suggestion", "code_snippet", "status", "verification_note", "trigger", "cascade_impact", "fix_alternatives", "confidence", "fix_strategy"],
```

- [ ] **Step 3: Validate JSON is well-formed**

Run: `python3 -c "import json; json.load(open('internal/schema/review.json'))"`
Expected: no output (success)

- [ ] **Step 4: Commit**

```bash
git add internal/schema/review.json
git commit -m "feat(schema): add confidence and fix_strategy to finding schema"
```

---

### Task 2: Add fields to Go structs + session versioning

**Files:**
- Modify: `internal/session/types.go`
- Test: `internal/session/manager_test.go`

- [ ] **Step 1: Write test for session version check**

Add to `internal/session/manager_test.go`:

```go
func TestManager_Load_RejectsOldVersion(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	// Create a session (will have current version)
	sess, err := mgr.Create([]string{"a.go"}, "files", "", cfg)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Manually overwrite with version 0 (simulates old session)
	sess.Version = 0
	if err := mgr.Update(sess); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Load should fail
	_, err = mgr.Load(sess.SessionID)
	if err == nil {
		t.Fatal("expected error loading old-version session")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("expected version error, got: %v", err)
	}
}
```

Add `"strings"` to the import block in manager_test.go.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -run TestManager_Load_RejectsOldVersion -v`
Expected: FAIL (Version field doesn't exist yet)

- [ ] **Step 3: Add Version to Session struct**

In `internal/session/types.go`, add `Version` field to `Session` struct after line 29:

```go
type Session struct {
	Version        int       `json:"version"`
	SessionID      string    `json:"session_id"`
	// ... rest unchanged
```

- [ ] **Step 4: Add Confidence and FixStrategy to Finding struct**

In `internal/session/types.go`, add after line 59 (after FixAlternatives):

```go
	Confidence      int              `json:"confidence"`
	FixStrategy     string           `json:"fix_strategy"`
```

- [ ] **Step 5: Add Confidence and FixStrategy to CodexFinding struct**

In `internal/session/types.go`, add after line 115 (after FixAlternatives):

```go
	Confidence      int              `json:"confidence"`
	FixStrategy     string           `json:"fix_strategy"`
```

Note: NO `omitempty` on CodexFinding — these are required fields from Codex.

- [ ] **Step 6: Add session version constant and set it in Create()**

In `internal/session/types.go`, add after the FindingReopened constant:

```go
// CurrentSessionVersion is incremented when the session schema changes.
const CurrentSessionVersion = 2
```

- [ ] **Step 7: Set version in Create()**

In `internal/session/manager.go`, in the `Create()` function, add `Version: CurrentSessionVersion,` to the Session literal (around line 42):

```go
	sess := &Session{
		Version:        CurrentSessionVersion,
		SessionID:      id,
		// ... rest unchanged
```

- [ ] **Step 8: Add version check in Load()**

In `internal/session/manager.go`, insert **before** line 82 (`return &sess, nil`),
after the unmarshal error check block (line 81):

```go
	if sess.Version != CurrentSessionVersion {
		return nil, fmt.Errorf("session %s uses schema version %d (current: %d); please start a new review",
			sessionID, sess.Version, CurrentSessionVersion)
	}
```

- [ ] **Step 8b: Write test for future version rejection**

Add to `internal/session/manager_test.go`:

```go
func TestManager_Load_RejectsFutureVersion(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	sess, err := mgr.Create([]string{"a.go"}, "files", "", cfg)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Simulate a session from a newer xreview version
	sess.Version = 999
	if err := mgr.Update(sess); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	_, err = mgr.Load(sess.SessionID)
	if err == nil {
		t.Fatal("expected error loading future-version session")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("expected version error, got: %v", err)
	}
}
```

- [ ] **Step 8c: Write test for confidence/fix_strategy round-trip**

Add to `internal/session/manager_test.go`:

```go
func TestManager_RoundTrip_ConfidenceAndFixStrategy(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	sess, _ := mgr.Create([]string{"a.go"}, "files", "", cfg)
	sess.Findings = []Finding{
		{
			ID: "F-001", Severity: "high", Category: "security",
			Status: FindingOpen, File: "a.go", Line: 1,
			Description: "test", Confidence: 85, FixStrategy: "auto",
		},
		{
			ID: "F-002", Severity: "medium", Category: "logic",
			Status: FindingOpen, File: "b.go", Line: 10,
			Description: "test2", Confidence: 40, FixStrategy: "ask",
		},
	}
	if err := mgr.Update(sess); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	loaded, err := mgr.Load(sess.SessionID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Findings[0].Confidence != 85 {
		t.Errorf("expected confidence 85, got %d", loaded.Findings[0].Confidence)
	}
	if loaded.Findings[0].FixStrategy != "auto" {
		t.Errorf("expected fix_strategy auto, got %s", loaded.Findings[0].FixStrategy)
	}
	if loaded.Findings[1].Confidence != 40 {
		t.Errorf("expected confidence 40, got %d", loaded.Findings[1].Confidence)
	}
	if loaded.Findings[1].FixStrategy != "ask" {
		t.Errorf("expected fix_strategy ask, got %s", loaded.Findings[1].FixStrategy)
	}
}
```

- [ ] **Step 9: Run tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v`
Expected: ALL PASS (including new TestManager_Load_RejectsOldVersion)

- [ ] **Step 10: Run full test suite to check no regressions**

Run: `cd /home/davidleitw/xreview && go test ./...`
Expected: ALL PASS

- [ ] **Step 11: Commit**

```bash
git add internal/session/types.go internal/session/manager.go internal/session/manager_test.go
git commit -m "feat(session): add confidence, fix_strategy fields and session versioning"
```

---

### Task 3: Add classification rules to Codex prompt

**Files:**
- Modify: `internal/prompt/templates.go`
- Test: `internal/prompt/builder_test.go`

- [ ] **Step 1: Write test for new prompt content**

Add to `internal/prompt/builder_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestBuildFirstRound_ContainsConfidenceInstructions -v`
Expected: FAIL (confidence not in template yet)

- [ ] **Step 3: Add classification rules to FirstRoundTemplate**

In `internal/prompt/templates.go`, append before the closing backtick of FirstRoundTemplate (line 49). Add after the fix_alternatives instruction:

```
- confidence: 0-100. How certain you are this is a real issue.
  100 = you can see the exact bug. 50 = it looks suspicious but you're not sure.
  0 = pure speculation. Be honest — overconfidence wastes the verifier's time.
- fix_strategy: "auto" or "ask".
  "auto" = a senior engineer would apply this fix without discussion:
    dead code, missing error check, obvious single-fix bug, stale comment.
  "ask" = reasonable engineers could disagree on the approach:
    security trade-offs, design decisions, behavior changes, multi-approach fixes,
    anything where confidence < 60.
  When in doubt, use "ask".
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestBuildFirstRound_ContainsConfidenceInstructions -v`
Expected: PASS

- [ ] **Step 5: Update ResumeTemplate inline JSON schema**

The ResumeTemplate (line 102-124 in templates.go) contains an inline JSON schema
example that Codex uses as format reference during resume rounds. This must include
the new fields. Add after the `fix_alternatives` line (line 121):

```
      "confidence": 85,
      "fix_strategy": "auto|ask"
```

- [ ] **Step 6: Run all prompt tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/prompt/templates.go internal/prompt/builder_test.go
git commit -m "feat(prompt): add confidence and fix_strategy to Codex prompt and resume schema"
```

---

### Task 4: Update FormatFindingsForPrompt for resume rounds

**Files:**
- Modify: `internal/prompt/builder.go`
- Test: `internal/prompt/builder_test.go`

- [ ] **Step 1: Write test for new format fields**

Add to `internal/prompt/builder_test.go`:

```go
func TestFormatFindingsForPrompt_IncludesConfidenceAndStrategy(t *testing.T) {
	b, err := NewBuilder()
	if err != nil {
		t.Fatal(err)
	}

	findings := []session.Finding{
		{
			ID:          "F-001",
			Severity:    "high",
			Category:    "security",
			File:        "main.go",
			Line:        42,
			Description: "SQL injection",
			Status:      "open",
			Confidence:  90,
			FixStrategy: "auto",
		},
	}

	result := b.FormatFindingsForPrompt(findings)
	assertContains(t, result, "confidence:90")
	assertContains(t, result, "strategy:auto")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestFormatFindingsForPrompt_IncludesConfidenceAndStrategy -v`
Expected: FAIL

- [ ] **Step 3: Update FormatFindingsForPrompt in builder.go**

In `internal/prompt/builder.go`, update the format string at line 82:

Change:
```go
		fmt.Fprintf(&buf, "[%s] (%s/%s) %s:%d — %s [status: %s]\n",
			f.ID, f.Severity, f.Category, f.File, f.Line, f.Description, f.Status)
```

To:
```go
		fmt.Fprintf(&buf, "[%s] (%s/%s, confidence:%d, strategy:%s) %s:%d — %s [status: %s]\n",
			f.ID, f.Severity, f.Category, f.Confidence, f.FixStrategy, f.File, f.Line, f.Description, f.Status)
```

- [ ] **Step 4: Run tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add internal/prompt/builder.go internal/prompt/builder_test.go
git commit -m "feat(prompt): include confidence and fix_strategy in resume findings format"
```

---

### Task 5: Update XML output + simplify agent instructions

**Files:**
- Modify: `internal/formatter/xml.go`
- Test: `internal/formatter/formatter_test.go`

- [ ] **Step 1: Write tests for new XML attributes and simplified agent instructions**

Add to `internal/formatter/formatter_test.go`:

```go
func TestFormatReviewResult_IncludesConfidenceAndFixStrategy(t *testing.T) {
	findings := []session.Finding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			Status:      "open",
			File:        "main.go",
			Line:        42,
			Description: "SQL injection",
			Confidence:  90,
			FixStrategy: "auto",
		},
	}
	summary := session.FindingSummary{Total: 1, Open: 1}

	result := FormatReviewResult("xr-test", 1, "REVISE", findings, summary)

	assertContains(t, result, `confidence="90"`)
	assertContains(t, result, `fix-strategy="auto"`)
}

func TestFormatReviewResult_AgentInstructionsSimplified(t *testing.T) {
	findings := []session.Finding{
		{
			ID: "F001", Severity: "high", Category: "security",
			Status: "open", File: "main.go", Line: 42,
			Description: "test", Confidence: 80, FixStrategy: "ask",
		},
	}
	summary := session.FindingSummary{Total: 1, Open: 1}

	result := FormatReviewResult("xr-test", 1, "REVISE", findings, summary)

	// Should have agent-instructions
	assertContains(t, result, "<agent-instructions>")
	// Should reference session ID
	assertContains(t, result, "xr-test")
	// Should point to skill instructions
	assertContains(t, result, "skill instructions")
	// Should NOT contain the old verbose Phase 1/2/3 workflow
	if strings.Contains(result, "PHASE 1: VERIFY FINDINGS") {
		t.Error("agent instructions should not contain verbose Phase 1 workflow — that belongs in SKILL.md")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -run "TestFormatReviewResult_Includes|TestFormatReviewResult_Agent" -v`
Expected: FAIL

- [ ] **Step 3: Add confidence and fix-strategy to XML finding output**

In `internal/formatter/xml.go`, update the finding opening tag at line 27-28.

Change:
```go
		fmt.Fprintf(&b, `  <finding id="%s" severity="%s" category="%s" status="%s">`+"\n",
			xmlEscape(f.ID), xmlEscape(f.Severity), xmlEscape(f.Category), xmlEscape(f.Status))
```

To:
```go
		fmt.Fprintf(&b, `  <finding id="%s" severity="%s" category="%s" status="%s" confidence="%d" fix-strategy="%s">`+"\n",
			xmlEscape(f.ID), xmlEscape(f.Severity), xmlEscape(f.Category), xmlEscape(f.Status), f.Confidence, xmlEscape(f.FixStrategy))
```

- [ ] **Step 4: Rewrite buildAgentInstructions to minimal version**

Replace the entire `buildAgentInstructions` function body (lines 80-166) with:

```go
func buildAgentInstructions(findings []session.Finding, summary session.FindingSummary, sessionID string) string {
	var b strings.Builder

	b.WriteString("<agent-instructions>\n")
	fmt.Fprintf(&b, "Session ID: %s\n", sessionID)
	fmt.Fprintf(&b, "Findings: %d total (%d open, %d fixed, %d dismissed)\n",
		summary.Total, summary.Open, summary.Fixed, summary.Dismissed)

	// Count by severity for context
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
```

- [ ] **Step 5: Run tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -v`
Expected: ALL PASS

- [ ] **Step 6: Run full test suite**

Run: `cd /home/davidleitw/xreview && go test ./...`
Expected: ALL PASS (some tests may need updating if they assert on old agent instruction content)

- [ ] **Step 7: Fix broken tests**

The following existing tests will break and need updating:

1. `TestFormatReviewResult_AgentInstructions_Present` — asserts on "PHASE 1: VERIFY FINDINGS",
   "PHASE 2: PRESENT ALL CONFIRMED FINDINGS", "PHASE 3: DISCUSSION & FIX GUIDANCE".
   → Update to assert on new content: "skill instructions", session ID, finding counts.

2. `TestBuildAgentInstructions_ContainsVerificationStep` — asserts on old Phase 1 content.
   → Update to assert on "skill instructions" and session ID.

3. `TestBuildAgentInstructions_GroupByFile` — asserts on old group-by-file instruction.
   → Update or remove (grouping logic moved to SKILL.md).

4. `TestBuildAgentInstructions_ReviewOnlyPresentation` — asserts on old Phase 2 content.
   → Update to assert on simplified output.

5. `TestFormatReviewResult_AgentInstructions_SeverityCounts` — severity count format changes.
   → Update to match new format ("High severity: N", "Auto-fixable: N").

6. `TestFormatReviewResult_WithFindings` and `TestFormatReviewResult_EnrichedFields` —
   add `Confidence` and `FixStrategy` to test Finding structs so XML output is meaningful
   (otherwise they'll have `confidence="0" fix-strategy=""`).

Run `go test ./internal/formatter/ -v` after each fix to verify incrementally.

- [ ] **Step 8: Commit**

```bash
git add internal/formatter/xml.go internal/formatter/formatter_test.go
git commit -m "feat(formatter): add confidence/fix-strategy to XML, simplify agent instructions"
```

---

### Task 6: Update SKILL.md with new workflow

**Files:**
- Modify: `~/.claude/plugins/marketplaces/xreview-marketplace/skills/review/SKILL.md`

This task replaces the current Step 2.5 through Step 4 with the new workflow
from the spec. Steps 0, 1, 2, 5, and the Important Notes section are unchanged.

- [ ] **Step 1: Replace Step 2.5 through Step 4**

Replace the section from `## Step 2.5: Verify + Fix Plan Gate (MANDATORY)` through
the end of `## Step 4: Summary + Verify` with the new workflow. The new content
implements the spec's Phase 1 (verify), Phase 2 (present with tiered format),
Phase 3 (user decision with adaptive options), Step 3 (batch execute), Step 3.5
(repair report), and Step 4 (single batch resume verification).

Key changes to encode in the skill:
- Phase 1 adds **OVERRIDE classification**: Claude Code can change Codex's fix_strategy
  (e.g., Codex said "auto" but Claude Code disagrees → override to "ask")
- Phase 2 uses **table format for auto findings** (one row per finding: location, problem, fix)
- Phase 2 uses **full analysis for ask findings** (7 sections: problem, root cause, trigger path, consequence, likelihood assessment, options, impact scope)
- **Likelihood assessment** requires Claude Code to think critically: is this realistic in production? Can the input be controlled? Is the trigger path reachable?
- If Claude Code cannot determine root cause or trigger path: write "Root cause unclear — Codex reports: [original]"
- Phase 3 AskUserQuestion **adapts to auto/ask mix** (different options when all-auto, all-ask, or mixed)
- Step 3.5 repair report is concise: 3 columns (Finding, Location, Fix Applied)
- Step 4 resumes once after all fixes, not per-finding

- [ ] **Step 2: Verify skill loads correctly**

Run: `cat ~/.claude/plugins/marketplaces/xreview-marketplace/skills/review/SKILL.md | head -12`
Expected: YAML frontmatter intact, name: xreview

- [ ] **Step 3: Commit**

```bash
cd ~/.claude/plugins/marketplaces/xreview-marketplace
git add skills/review/SKILL.md
git commit -m "feat(skill): redesign review workflow with tiered presentation and batch fixes"
```

---

### Task 7: Update reference.md with new XML attributes

**Files:**
- Modify: `~/.claude/plugins/marketplaces/xreview-marketplace/skills/review/reference.md`

- [ ] **Step 1: Add confidence and fix-strategy to finding element documentation**

Add the new attributes to the finding element reference, documenting that
`confidence` is an integer 0-100 and `fix-strategy` is "auto" or "ask".

- [ ] **Step 2: Commit**

```bash
cd ~/.claude/plugins/marketplaces/xreview-marketplace
git add skills/review/reference.md
git commit -m "docs(skill): add confidence and fix-strategy to XML schema reference"
```

---

### Task 8: Integration verification

- [ ] **Step 1: Run full test suite**

Run: `cd /home/davidleitw/xreview && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Build binary**

Run: `cd /home/davidleitw/xreview && go build ./cmd/xreview/`
Expected: Success, no errors

- [ ] **Step 3: Run preflight check**

Run: `./xreview preflight`
Expected: success (confirms binary works)

- [ ] **Step 4: Verify schema is valid by checking it loads**

Run: `./xreview version`
Expected: Version output (confirms binary is functional)

- [ ] **Step 5: Clean up build artifact**

Run: `rm -f xreview`
