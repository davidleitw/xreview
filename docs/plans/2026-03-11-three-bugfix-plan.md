# Three Bugfix Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix three bugs: error misclassification, resume+files UX gap, and resume JSON parse failure.

**Architecture:** All three fixes are independent and can be implemented in parallel. Fix 1 tightens a string match. Fix 2 relaxes CLI validation and threads extra files through Verify(). Fix 3 adds JSON format instruction to the resume prompt and hardens the JSON extractor.

**Tech Stack:** Go, text/template, cobra CLI

**Spec:** `docs/specs/2026-03-11-three-bugfix-design.md`

---

## Chunk 1: Fix 1 — Tighten `classifyReviewError()` + Fix 3 — ExtractJSON fallback + ResumeTemplate JSON instruction

### Task 1: Tighten `classifyReviewError()` string match

**Files:**
- Modify: `cmd/xreview/cmd_review.go:149`
- Modify: `cmd/xreview/cmd_review_test.go`

- [ ] **Step 1: Write failing test for the false-positive case**

Add to `cmd/xreview/cmd_review_test.go`:

```go
func TestClassifyReviewError_DoesNotFalsePositiveOnCodexStderr(t *testing.T) {
	// Simulates codex failing with stderr that contains "session id: ..."
	err := fmt.Errorf("codex exited with error: exit status 1\nstderr: session id: 019cdb8c-6b73-79e3-8860-190f58f25ddc")
	code := classifyReviewError(err)
	if code == "SESSION_NOT_FOUND" {
		t.Errorf("should not classify codex stderr as SESSION_NOT_FOUND, got %s", code)
	}
}

func TestClassifyReviewError_RealSessionNotFound(t *testing.T) {
	// Simulates actual session-not-found from session/manager.go
	err := fmt.Errorf("load session: session %q not found", "xr-20260311-abc123")
	code := classifyReviewError(err)
	if code != "SESSION_NOT_FOUND" {
		t.Errorf("expected SESSION_NOT_FOUND, got %s", code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./cmd/xreview/ -run TestClassifyReviewError -v`
Expected: `TestClassifyReviewError_DoesNotFalsePositiveOnCodexStderr` FAILS

- [ ] **Step 3: Fix the string match**

In `cmd/xreview/cmd_review.go`, change line 149 from:

```go
case strings.Contains(msg, "session") && strings.Contains(msg, "not found"):
```

to:

```go
case strings.Contains(msg, "\" not found"):
```

This matches `session "xr-..." not found` (the quoted format from `manager.go:73`) but does NOT match `session id: <uuid>` from codex stderr.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /home/davidleitw/xreview && go test ./cmd/xreview/ -run TestClassifyReviewError -v`
Expected: PASS

- [ ] **Step 5: Also update the existing flag validation test**

In `cmd/xreview/cmd_review_test.go`, the test case `"files with session"` at line 50-53 expects the old error `"--files/--git-uncommitted cannot be used with --session"`. This test will be **removed** in Task 3 (Fix 2), but for now ensure it still passes:

Run: `cd /home/davidleitw/xreview && go test ./cmd/xreview/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/xreview/cmd_review.go cmd/xreview/cmd_review_test.go
git commit -m "fix: tighten classifyReviewError to avoid false SESSION_NOT_FOUND"
```

---

### Task 2: Add fallback JSON extraction in `ExtractJSON`

**Files:**
- Modify: `internal/parser/extract.go:15-47`
- Modify: `internal/parser/parser_test.go`

- [ ] **Step 1: Write failing test for embedded JSON**

Add to `internal/parser/parser_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/parser/ -run TestExtractJSON_Embedded -v`
Expected: FAIL with "could not extract JSON from output"

- [ ] **Step 3: Add fallback extraction logic**

In `internal/parser/extract.go`, replace the `ExtractJSON` function with:

```go
func ExtractJSON(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("empty output")
	}

	// Try direct parse first (--output-schema should produce clean JSON)
	if strings.HasPrefix(trimmed, "{") {
		return trimmed, nil
	}

	// Fallback 1: strip markdown code fences
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		var jsonLines []string
		inside := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inside = !inside
				continue
			}
			if inside {
				jsonLines = append(jsonLines, line)
			}
		}
		result := strings.Join(jsonLines, "\n")
		if strings.TrimSpace(result) != "" {
			return result, nil
		}
	}

	// Fallback 2: find the first '{' and extract the outermost JSON object
	// by matching braces. This handles cases where codex wraps JSON in prose.
	if idx := strings.Index(trimmed, "{"); idx >= 0 {
		candidate := trimmed[idx:]
		if end := findClosingBrace(candidate); end > 0 {
			return candidate[:end+1], nil
		}
	}

	return "", fmt.Errorf("could not extract JSON from output")
}

// findClosingBrace finds the index of the closing '}' that matches the
// opening '{' at position 0, accounting for nesting and JSON strings.
// Returns -1 if no matching brace is found.
func findClosingBrace(s string) int {
	depth := 0
	inString := false
	escaped := false
	for i, ch := range s {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
```

- [ ] **Step 4: Run all parser tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/parser/ -v`
Expected: All PASS (including old tests and new embedded tests)

- [ ] **Step 5: Commit**

```bash
git add internal/parser/extract.go internal/parser/parser_test.go
git commit -m "fix: add fallback JSON extraction for resume output"
```

---

### Task 3: Add JSON format instruction to `ResumeTemplate`

**Files:**
- Modify: `internal/prompt/templates.go:36-59`
- Modify: `internal/prompt/builder.go:19-23`
- Modify: `internal/prompt/builder_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/prompt/builder_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run "TestBuildResume_(ContainsJSON|WithAdditional|NoAdditional)" -v`
Expected: FAIL — no `"verdict"` in output, no `AdditionalFiles` field

- [ ] **Step 3: Add `AdditionalFiles` to `ResumeInput`**

In `internal/prompt/builder.go`, change the `ResumeInput` struct (line 19-23) to:

```go
type ResumeInput struct {
	Message          string
	PreviousFindings string
	UpdatedFiles     string
	AdditionalFiles  string // optional: extra files added via --files on resume
}
```

- [ ] **Step 4: Update `ResumeTemplate` with JSON instruction and optional additional files section**

In `internal/prompt/templates.go`, replace `ResumeTemplate` (lines 36-59) with:

```go
const ResumeTemplate = `This is a follow-up review. You previously reviewed these files and
identified the findings listed below. The developer has made changes
and provided the following update:

Developer message: "{{.Message}}"

===== PREVIOUS FINDINGS =====

{{.PreviousFindings}}

===== UPDATED FILES =====

{{.UpdatedFiles}}

===== END OF FILES =====
{{if .AdditionalFiles}}
===== ADDITIONAL FILES =====

The developer has requested you also review these additional files:

{{.AdditionalFiles}}

===== END OF ADDITIONAL FILES =====
{{end}}
For each previous finding, determine:
1. If claimed fixed: verify the fix is actually correct and complete.
2. If claimed false positive: evaluate whether the dismissal is reasonable.
3. If no update: re-evaluate against the current code.

Also check: did any of the changes introduce NEW issues?

New findings (not in the previous list) should have status "open" and a new unique "id".

Respond with ONLY a JSON object (no markdown fences, no explanation before or after).
Use this exact schema:
{
  "verdict": "APPROVED or REVISE",
  "summary": "brief summary of your review",
  "findings": [
    {
      "id": "F-001",
      "severity": "high|medium|low",
      "category": "security|logic|performance|error-handling",
      "file": "path/to/file",
      "line": 42,
      "description": "what is wrong",
      "suggestion": "how to fix it",
      "code_snippet": "the relevant code",
      "status": "open|fixed|dismissed|reopened",
      "verification_note": "verification details or empty string"
    }
  ]
}`
```

- [ ] **Step 5: Run all prompt tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/prompt/templates.go internal/prompt/builder.go internal/prompt/builder_test.go
git commit -m "fix: add JSON format instruction to ResumeTemplate and AdditionalFiles support"
```

---

## Chunk 2: Fix 2 — Allow `--session` + `--files`

### Task 4: Relax CLI validation and thread extra targets to Verify

**Files:**
- Modify: `cmd/xreview/cmd_review.go:32-54,75-90`
- Modify: `cmd/xreview/cmd_review_test.go`
- Modify: `internal/reviewer/reviewer.go:26-32`
- Modify: `internal/reviewer/single.go:119-165`

- [ ] **Step 1: Update `VerifyRequest` to accept extra targets**

In `internal/reviewer/reviewer.go`, change `VerifyRequest` (lines 27-32) to:

```go
type VerifyRequest struct {
	SessionID       string
	Message         string
	FullRescan      bool
	Timeout         int
	ExtraTargets    []string // additional files to include in resume
	ExtraTargetMode string   // "files" or "git-uncommitted" for extra targets
}
```

- [ ] **Step 2: Update `PreRunE` validation**

In `cmd/xreview/cmd_review.go`, replace the `PreRunE` block (lines 32-54) with:

```go
PreRunE: func(cmd *cobra.Command, args []string) error {
	hasFiles := files != ""
	hasGit := gitUncommitted
	hasSession := sessionID != ""

	if hasFiles && hasGit {
		return fmt.Errorf("--files and --git-uncommitted are mutually exclusive")
	}
	if !hasSession && !hasFiles && !hasGit {
		return fmt.Errorf("new review requires --files or --git-uncommitted")
	}
	if !hasSession && message != "" {
		return fmt.Errorf("--message requires --session")
	}
	if !hasSession && fullRescan {
		return fmt.Errorf("--full-rescan requires --session")
	}

	return nil
},
```

(The `hasSession && (hasFiles || hasGit)` check is removed.)

- [ ] **Step 3: Update `RunE` to pass extra targets on resume**

In `cmd/xreview/cmd_review.go`, replace the session branch in `RunE` (lines 75-90) with:

```go
if sessionID != "" {
	req := reviewer.VerifyRequest{
		SessionID:  sessionID,
		Message:    message,
		FullRescan: fullRescan,
		Timeout:    timeout,
	}

	// Pass extra files/git targets if provided
	if gitUncommitted {
		req.ExtraTargetMode = "git-uncommitted"
	} else if files != "" {
		req.ExtraTargets = splitTargets(files)
		req.ExtraTargetMode = "files"
	}

	result, err := rev.Verify(cmd.Context(), req)
	if err != nil {
		return printErr("review", classifyReviewError(err), err)
	}
	fmt.Println(formatter.FormatReviewResult(
		result.SessionID, result.Round, result.Verdict,
		result.Findings, result.Summary,
	))
	return nil
}
```

- [ ] **Step 4: Update `Verify()` to collect and include extra files**

In `internal/reviewer/single.go`, replace the `Verify` method (lines 119-197) with:

```go
func (r *SingleReviewer) Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	// 1. Load session
	sess, err := r.sessions.Load(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}

	// 2. Collect files from original session targets
	files, err := r.collector.Collect(ctx, sess.Targets, sess.TargetMode)
	if err != nil {
		return nil, err
	}

	// 3. Collect extra files if provided
	var additionalContent string
	if len(req.ExtraTargets) > 0 || req.ExtraTargetMode == "git-uncommitted" {
		extraFiles, err := r.collector.Collect(ctx, req.ExtraTargets, req.ExtraTargetMode)
		if err != nil {
			return nil, err
		}
		_, additionalContent = formatFilesForPrompt(extraFiles)
	}

	// 4. Build resume prompt
	_, updatedFiles := formatFilesForPrompt(files)
	promptStr, err := r.builder.BuildResume(prompt.ResumeInput{
		Message:          req.Message,
		PreviousFindings: r.builder.FormatFindingsForPrompt(sess.Findings),
		UpdatedFiles:     updatedFiles,
		AdditionalFiles:  additionalContent,
	})
	if err != nil {
		return nil, fmt.Errorf("build resume prompt: %w", err)
	}

	// 5. Write temp schema
	schemaPath, cleanup, err := schema.WriteTempSchema()
	if err != nil {
		return nil, fmt.Errorf("write schema: %w", err)
	}
	defer cleanup()

	// 6. Determine resume vs fresh
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = time.Duration(r.cfg.DefaultTimeout) * time.Second
	}

	execReq := codex.ExecRequest{
		Model:      r.cfg.CodexModel,
		Prompt:     promptStr,
		SchemaPath: schemaPath,
		Timeout:    timeout,
	}
	if codex.ShouldResume(sess, req.FullRescan) {
		execReq.ResumeSessionID = sess.CodexSessionID
	}

	execResult, err := r.runner.Exec(ctx, execReq)
	if err != nil {
		return nil, err
	}

	// 7. Parse response
	codexResp, err := r.parser.Parse(execResult.Stdout)
	if err != nil {
		return nil, fmt.Errorf("parse codex output: %w", err)
	}

	// 8. Merge findings
	sess.Round++
	sess.Status = StatusVerifying
	if execResult.CodexSessionID != "" {
		sess.CodexSessionID = execResult.CodexSessionID
	}
	sess.Findings = mergeFindings(sess.Findings, codexResp.Findings)

	if err := r.sessions.Update(sess); err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	summary := sess.Summarize()
	return &VerifyResult{
		SessionID: sess.SessionID,
		Round:     sess.Round,
		Verdict:   codexResp.Verdict,
		Findings:  sess.Findings,
		Summary:   summary,
	}, nil
}
```

- [ ] **Step 5: Update CLI validation tests**

In `cmd/xreview/cmd_review_test.go`, replace the test case at lines 50-53:

```go
// Remove this test case:
// {
//     name:    "files with session",
//     args:    []string{"--files", "a.go", "--session", "s1"},
//     wantErr: "--files/--git-uncommitted cannot be used with --session",
// },
```

And add a new test verifying `--session` + `--files` is accepted at the validation level:

```go
func TestReviewCmd_SessionWithFilesAllowed(t *testing.T) {
	root := newRootCmd()
	// This should pass PreRunE validation (will fail later in RunE because
	// session doesn't exist, but that's expected — we're testing flag validation)
	root.SetArgs([]string{"review", "--session", "xr-fake", "--files", "a.go", "--message", "check these too"})
	err := root.Execute()
	if err == nil {
		t.Skip("no real session, but PreRunE should have passed")
	}
	// Should NOT be the old mutual-exclusion error
	if strings.Contains(err.Error(), "--files/--git-uncommitted cannot be used with --session") {
		t.Error("--session + --files should now be allowed")
	}
}
```

- [ ] **Step 6: Run all tests**

Run: `cd /home/davidleitw/xreview && go test ./... -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/xreview/cmd_review.go cmd/xreview/cmd_review_test.go internal/reviewer/reviewer.go internal/reviewer/single.go
git commit -m "feat: allow --session with --files to add extra files on resume"
```

---

## Chunk 3: Final verification

### Task 5: Full test suite and cleanup

- [ ] **Step 1: Run full test suite**

Run: `cd /home/davidleitw/xreview && go test ./... -v -count=1`
Expected: All PASS

- [ ] **Step 2: Build check**

Run: `cd /home/davidleitw/xreview && go build ./...`
Expected: No errors

- [ ] **Step 3: Vet check**

Run: `cd /home/davidleitw/xreview && go vet ./...`
Expected: No warnings

- [ ] **Step 4: Delete the test script**

Run: `rm /tmp/xreview-test/test_codex_resume_schema.py`

- [ ] **Step 5: Final commit if any fixups needed**

```bash
git add -A
git commit -m "chore: cleanup after three-bugfix implementation"
```
