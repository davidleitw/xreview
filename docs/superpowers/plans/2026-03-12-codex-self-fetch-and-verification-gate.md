# Codex Self-Fetch + Claude Code Verification Gate — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** (1) Eliminate context overflow by letting Codex fetch diffs itself instead of xreview stuffing them into the prompt. (2) Make Claude Code verify Codex findings before presenting to user, with the ability to challenge suspect findings via xreview.

**Architecture:** Two independent changes. Change 1 replaces the "collect files → stuff prompt" pipeline with a lightweight instruction-only prompt that tells Codex what git commands to run. Change 2 modifies agent-instructions to require Claude Code to independently verify findings before presenting the Fix Plan, and to use `xreview review --session <id> --message "..."` to challenge suspect findings with Codex.

**Tech Stack:** Go (xreview CLI), Markdown (SKILL.md)

---

## Chunk 1: Codex Self-Fetch — Let Codex Get Its Own Diff

### Task 1: Rewrite FirstRoundTemplate to instruction-only prompt

**Files:**
- Modify: `internal/prompt/templates.go:4-43`

The core change: remove `{{.Diff}}` from the template. Instead, tell Codex what git/file commands to run to get the code.

- [ ] **Step 1: Write the failing test**

Add test in `internal/prompt/builder_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestBuildFirstRound_InstructionOnly -v`
Expected: FAIL — `FirstRoundInput` has no `FetchMethod` field

- [ ] **Step 3: Update FirstRoundInput struct**

In `internal/prompt/builder.go`, change `FirstRoundInput`:

```go
type FirstRoundInput struct {
	Context     string
	FetchMethod string // git command or file list instruction for Codex to run
	FileList    string // summary of files (names + line counts)
}
```

Remove the `Diff` field.

- [ ] **Step 4: Rewrite FirstRoundTemplate**

In `internal/prompt/templates.go`, replace `FirstRoundTemplate`:

```go
const FirstRoundTemplate = `<CRITICAL_RULES>
1. PERFORM STATIC ANALYSIS ONLY. Do NOT execute or run the code.
2. Only report issues you can directly observe in the code.
   Do NOT speculate about issues in code you cannot see.
3. Every finding MUST reference a specific file and line number.
4. Focus on real bugs and security issues. Do NOT report trivial style preferences.
5. If you find no issues, set verdict to APPROVED with an empty findings array.
6. You MUST read additional files in the repository to understand the full context.
7. Review comprehensively: security, correctness, readability, maintainability,
   and extensibility. Do NOT limit your review to a single aspect.
8. Suggestions MUST be scoped and actionable within the current change.
   Do NOT suggest large-scale rewrites or architectural overhauls.
</CRITICAL_RULES>

You are a senior code reviewer. Analyze the code for bugs,
security vulnerabilities, logic errors, and significant quality issues.

Context from the developer: {{.Context}}

===== HOW TO GET THE CODE =====

{{.FetchMethod}}

Files involved:

{{.FileList}}

You MUST follow the instructions above to get the actual code.
Read additional files as needed for full context (callers, callees, type definitions, etc.).
Pay close attention to the developer context — it tells you what to focus on.

===== END =====

For each finding, you MUST also provide these fields:
- trigger: the concrete input, scenario, or call sequence that manifests the issue.
  Be specific (e.g. "user sends id=' OR '1'='1") not abstract (e.g. "malicious input").
- cascade_impact: other files/functions in the repository that would be affected if
  this code is changed. Trace the call chain. Use format "file:function() — description".
  You are encouraged to read additional files to identify these. Empty array [] if none.
- fix_alternatives: provide 2-3 fix approaches. Each has label (A/B/C), description,
  effort (minimal/moderate/large), and recommended (true for exactly one).
  Consider trade-offs: minimal fix vs. systemic improvement.`
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestBuildFirstRound_InstructionOnly -v`
Expected: PASS

- [ ] **Step 6: Fix existing tests that reference the old Diff field**

Update `TestBuildFirstRound` to use `FetchMethod` instead of `Diff`:

```go
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
```

- [ ] **Step 7: Run all prompt tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -v`
Expected: ALL PASS

- [ ] **Step 8: Commit**

```bash
git add internal/prompt/templates.go internal/prompt/builder.go internal/prompt/builder_test.go
git commit -m "feat: rewrite FirstRoundTemplate to instruction-only (Codex self-fetch)"
```

### Task 2: Rewrite ResumeTemplate to instruction-only

**Files:**
- Modify: `internal/prompt/templates.go:46-103`

Same pattern: remove `{{.UpdatedFiles}}` and `{{.AdditionalFiles}}` content embedding. Tell Codex to re-read the files itself.

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -run TestBuildResume_InstructionOnly -v`
Expected: FAIL

- [ ] **Step 3: Update ResumeInput struct**

In `internal/prompt/builder.go`:

```go
type ResumeInput struct {
	Message          string
	PreviousFindings string
	FetchMethod      string // git command for Codex to re-read files
	FileList         string // summary of files involved
}
```

Remove `UpdatedFiles` and `AdditionalFiles` fields.

- [ ] **Step 4: Rewrite ResumeTemplate**

```go
const ResumeTemplate = `This is a follow-up review. You previously reviewed these files and
identified the findings listed below. The developer has made changes
and provided the following update:

Developer message: "{{.Message}}"

===== PREVIOUS FINDINGS =====

{{.PreviousFindings}}

===== HOW TO GET THE UPDATED CODE =====

{{.FetchMethod}}

Files involved:

{{.FileList}}

You MUST follow the instructions above to get the current code.
Read additional files as needed for full context.

===== END =====

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
      "verification_note": "verification details or empty string",
      "trigger": "concrete trigger condition",
      "cascade_impact": ["file:func() — impact description"],
      "fix_alternatives": [
        {"label": "A", "description": "fix approach", "effort": "minimal|moderate|large", "recommended": true}
      ]
    }
  ]
}`
```

- [ ] **Step 5: Fix existing resume tests**

Update `TestBuildResume`, `TestBuildResume_WithAdditionalFiles`, `TestBuildResume_NoAdditionalFiles` to use the new `ResumeInput` struct. Remove tests for `AdditionalFiles` (no longer a concept — Codex reads files itself).

- [ ] **Step 6: Run all prompt tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/prompt/templates.go internal/prompt/builder.go internal/prompt/builder_test.go
git commit -m "feat: rewrite ResumeTemplate to instruction-only (Codex self-fetch)"
```

### Task 3: Update SingleReviewer.Review() to build FetchMethod instead of collecting file content

**Files:**
- Modify: `internal/reviewer/single.go:47-117`
- Modify: `internal/reviewer/types.go` (if exists, otherwise fields are in single.go)

The reviewer no longer needs to collect file content for the prompt. It only needs to:
1. Determine the right `FetchMethod` string (git command)
2. Build a `FileList` summary (just file names, no content)

- [ ] **Step 1: Write the failing test**

In `internal/reviewer/single_test.go`:

```go
func TestReview_NoFileContentInPrompt(t *testing.T) {
	mgr := newMockManager()
	coll := &mockCollector{
		files: []collector.FileContent{
			{Path: "main.go", Content: "package main\n", Lines: 1},
		},
	}

	var capturedPromptInput prompt.FirstRoundInput
	bldr := &mockBuilder{
		firstRoundFn: func(input prompt.FirstRoundInput) (string, error) {
			capturedPromptInput = input
			return "prompt", nil
		},
	}

	runner := &mockRunner{
		execFn: func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
			return &codex.ExecResult{
				Stdout:         "{}",
				CodexSessionID: "codex-sess-789",
			}, nil
		},
	}
	psr := &mockParser{
		parseFn: func(stdout string) (*session.CodexResponse, error) {
			return &session.CodexResponse{Verdict: "APPROVED"}, nil
		},
	}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 180}

	r := NewSingleReviewer(runner, bldr, psr, mgr, coll, cfg)

	_, err := r.Review(context.Background(), ReviewRequest{
		Targets:    []string{"main.go"},
		TargetMode: "files",
		Context:    "test context",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FetchMethod should instruct Codex to read files (not git diff)
	if capturedPromptInput.FetchMethod == "" {
		t.Error("expected FetchMethod to be set")
	}
	if !strings.Contains(capturedPromptInput.FetchMethod, "main.go") {
		t.Errorf("expected FetchMethod to reference target file, got: %s", capturedPromptInput.FetchMethod)
	}
	if strings.Contains(capturedPromptInput.FetchMethod, "git diff") {
		t.Error("files mode should not use git diff")
	}
	if capturedPromptInput.FileList == "" {
		t.Error("expected FileList to be set")
	}
}

func TestReview_GitUncommitted_UseGitDiff(t *testing.T) {
	mgr := newMockManager()
	coll := &mockCollector{
		files: []collector.FileContent{
			{Path: "main.go", Content: "package main\n", Lines: 1},
		},
	}

	var capturedPromptInput prompt.FirstRoundInput
	bldr := &mockBuilder{
		firstRoundFn: func(input prompt.FirstRoundInput) (string, error) {
			capturedPromptInput = input
			return "prompt", nil
		},
	}

	runner := &mockRunner{
		execFn: func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
			return &codex.ExecResult{Stdout: "{}", CodexSessionID: "codex-sess-789"}, nil
		},
	}
	psr := &mockParser{
		parseFn: func(stdout string) (*session.CodexResponse, error) {
			return &session.CodexResponse{Verdict: "APPROVED"}, nil
		},
	}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 180}

	r := NewSingleReviewer(runner, bldr, psr, mgr, coll, cfg)

	_, err := r.Review(context.Background(), ReviewRequest{
		TargetMode: "git-uncommitted",
		Context:    "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// git-uncommitted mode should use git diff commands
	if !strings.Contains(capturedPromptInput.FetchMethod, "git diff") {
		t.Errorf("expected git diff in FetchMethod, got: %s", capturedPromptInput.FetchMethod)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/reviewer/ -run TestReview_NoFileContentInPrompt -v`
Expected: FAIL — `FirstRoundInput` no longer has `Diff`

- [ ] **Step 3: Add FetchMethod builder logic**

In `internal/reviewer/single.go`, add a helper and update `Review()`:

```go
// buildFetchMethod constructs the instruction for Codex to get the code to review.
// Two distinct modes:
//   - "git-uncommitted": Codex runs git diff commands to see uncommitted changes
//   - "files": Codex reads the specified files directly (no git involved —
//     supports use cases like reviewing a single file's quality or tracing
//     a flow across multiple files described by --context)
func buildFetchMethod(targets []string, targetMode string) string {
	switch targetMode {
	case "git-uncommitted":
		return "Run these commands to see the uncommitted changes:\n" +
			"  git diff          # unstaged changes\n" +
			"  git diff --cached # staged changes\n" +
			"  git ls-files --others --exclude-standard  # untracked files (read their content too)"
	case "files":
		var b strings.Builder
		b.WriteString("Read the following files in full and review them:\n")
		for _, t := range targets {
			fmt.Fprintf(&b, "  - %s\n", t)
		}
		b.WriteString("\nThese are NOT necessarily git changes — you are reviewing the files themselves.\n")
		b.WriteString("Use the developer's --context description to understand what to focus on.")
		return b.String()
	default:
		return "git diff HEAD"
	}
}

// buildFileListSummary creates a brief summary of file names for the prompt.
func buildFileListSummary(files []collector.FileContent) string {
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "%s (%d lines)\n", f.Path, f.Lines)
	}
	return b.String()
}
```

Update `Review()` to use these:

```go
func (r *SingleReviewer) Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	// 1. Create session
	sess, err := r.sessions.Create(req.Targets, req.TargetMode, req.Context, r.cfg)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// 2. Collect file metadata (for file list summary only)
	files, err := r.collector.Collect(ctx, req.Targets, req.TargetMode)
	if err != nil {
		return nil, err
	}

	// 3. Build prompt (instruction-only, no file content)
	promptStr, err := r.builder.BuildFirstRound(prompt.FirstRoundInput{
		Context:     req.Context,
		FetchMethod: buildFetchMethod(req.Targets, req.TargetMode),
		FileList:    buildFileListSummary(files),
	})
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	// ... rest unchanged (schema, codex call, parse, update session) ...
}
```

- [ ] **Step 4: Update Verify() similarly**

```go
func (r *SingleReviewer) Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	sess, err := r.sessions.Load(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}

	// Collect file metadata for summary
	files, err := r.collector.Collect(ctx, sess.Targets, sess.TargetMode)
	if err != nil {
		return nil, err
	}

	// Build fetch method — same mode as original review
	// For "files" mode: re-read the same files
	// For "git-uncommitted": re-run git diff to see current state
	fetchMethod := buildFetchMethod(sess.Targets, sess.TargetMode)

	promptStr, err := r.builder.BuildResume(prompt.ResumeInput{
		Message:          req.Message,
		PreviousFindings: r.builder.FormatFindingsForPrompt(sess.Findings),
		FetchMethod:      fetchMethod,
		FileList:         buildFileListSummary(files),
	})
	if err != nil {
		return nil, fmt.Errorf("build resume prompt: %w", err)
	}

	// ... rest unchanged ...
}
```

- [ ] **Step 5: Remove formatFilesForPrompt helper**

Delete the `formatFilesForPrompt` function (no longer used). Update or remove `TestFormatFilesForPrompt`.

- [ ] **Step 6: Fix existing reviewer tests**

Update mock builders to use the new `FirstRoundInput`/`ResumeInput` structs. Update `TestVerify_HappyPath` to not check for file content in prompts.

- [ ] **Step 7: Run all reviewer tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/reviewer/ -v`
Expected: ALL PASS

- [ ] **Step 8: Commit**

```bash
git add internal/reviewer/single.go internal/reviewer/single_test.go
git commit -m "feat: reviewer builds FetchMethod instead of embedding file content"
```

### Task 4: Simplify collector — metadata only option

**Files:**
- Modify: `internal/collector/collector.go`

Now that the reviewer only needs file metadata (path + line count) for the prompt, the collector still reads file content to compute line counts but this content is no longer passed to the prompt. This is fine for now — the collector stays as-is. No changes needed unless we want to optimize.

**Decision: Skip this task.** The collector is small and reads file content is fast. The performance gain of a metadata-only mode is negligible. The real win (eliminating context overflow) comes from not putting content into the prompt.

### Task 5: Run full test suite and verify

- [ ] **Step 1: Run all tests**

Run: `cd /home/davidleitw/xreview && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Build binary**

Run: `cd /home/davidleitw/xreview && go build ./cmd/xreview/`
Expected: SUCCESS

- [ ] **Step 3: Commit if any fixups needed**

---

## Chunk 2: Claude Code Verification Gate — Verify Before Presenting

### Task 6: Update agent-instructions to require verification

**Files:**
- Modify: `internal/formatter/xml.go:78-127`

Change `buildAgentInstructions()` to instruct Claude Code to **verify each finding** by reading the actual code before presenting the Fix Plan.

- [ ] **Step 1: Write the failing test**

In `internal/formatter/formatter_test.go`, add:

```go
func TestBuildAgentInstructions_ContainsVerificationStep(t *testing.T) {
	findings := []session.Finding{
		{ID: "F001", Severity: "high", Status: "open"},
	}
	summary := session.FindingSummary{Total: 1, Open: 1}

	result := FormatReviewResult("xr-test", 1, "REVISE", findings, summary)

	assertContains(t, result, "VERIFY each finding")
	assertContains(t, result, "Read the actual source code")
	assertContains(t, result, "confirmed")
	assertContains(t, result, "suspect")
	assertContains(t, result, "xreview review --session")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -run TestBuildAgentInstructions_ContainsVerificationStep -v`
Expected: FAIL

- [ ] **Step 3: Rewrite buildAgentInstructions()**

```go
func buildAgentInstructions(findings []session.Finding, summary session.FindingSummary, sessionID string) string {
	var b strings.Builder

	b.WriteString("<agent-instructions>\n")

	// Phase 1: Verification
	b.WriteString("== PHASE 1: VERIFY FINDINGS ==\n\n")
	b.WriteString("Before presenting ANY finding to the user, you MUST VERIFY each one yourself.\n")
	b.WriteString("Do NOT blindly trust Codex findings. You are an independent reviewer.\n\n")

	b.WriteString("For EACH finding:\n")
	b.WriteString("1. Read the actual source code at the specified file:line\n")
	b.WriteString("2. Analyze whether the finding is valid given the full code context\n")
	b.WriteString("   - For concurrency/lock findings: verify lock scope (nested vs sequential),\n")
	b.WriteString("     actual contention scenarios, and whether locks are truly held simultaneously\n")
	b.WriteString("   - For logic findings: trace the actual code path, verify the trigger scenario\n")
	b.WriteString("   - For security findings: confirm the input actually reaches the vulnerable code\n")
	b.WriteString("3. Mark each finding as:\n")
	b.WriteString("   - CONFIRMED: you verified the issue exists in the code\n")
	b.WriteString("   - SUSPECT: you believe this may be a false positive, with your reasoning\n\n")

	b.WriteString("For SUSPECT findings, you MUST challenge Codex before presenting to user:\n")
	fmt.Fprintf(&b, "  Run: xreview review --session %s --message \"F-XXX: I believe this is a false positive because [your reasoning]. Please re-evaluate.\"\n", sessionID)
	b.WriteString("  Parse Codex's response. If Codex agrees, drop the finding. If Codex disagrees\n")
	b.WriteString("  with valid reasoning, mark as CONFIRMED. If Codex disagrees without strong\n")
	b.WriteString("  reasoning, present both perspectives to the user and let them decide.\n\n")

	// Phase 2: Fix Plan
	b.WriteString("== PHASE 2: FIX PLAN (only CONFIRMED findings) ==\n\n")
	b.WriteString("Present ALL confirmed findings as a Fix Plan BEFORE making any code changes.\n")
	b.WriteString("This is a hard gate — do NOT skip it, do NOT start fixing.\n\n")

	b.WriteString("For EACH confirmed finding, include:\n")
	b.WriteString("1. Header: ### F-XXX: title (category/severity) with file:line\n")
	b.WriteString("2. Trigger: the <trigger> content — verified by your own code reading\n")
	b.WriteString("3. Impact: what happens if exploited/triggered\n")
	b.WriteString("4. Cascade: list every <impact> from <cascade-impact> — these show what else breaks\n")
	b.WriteString("5. Fix options: list ALL <alternative> entries, mark which is recommended.\n")
	b.WriteString("   ALWAYS add a final option: \"Don't fix — risk: <consequence>\"\n\n")

	// Severity counts
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
		fmt.Fprintf(&b, "⚠ %d HIGH severity finding(s) — verify with extra care, present full analysis.\n", highCount)
	}
	if mediumCount > 0 {
		fmt.Fprintf(&b, "%d MEDIUM severity finding(s) — verify and include analysis.\n", mediumCount)
	}
	if lowCount > 0 {
		fmt.Fprintf(&b, "%d LOW severity finding(s) — verify, brief description with fix options.\n", lowCount)
	}

	b.WriteString("\nAfter listing ALL confirmed findings, you MUST use AskUserQuestion with these options:\n")
	b.WriteString("  A. Execute all recommended fixes\n")
	b.WriteString("  B. Only fix high severity, skip the rest\n")
	b.WriteString("  C. I want to adjust (tell me which findings to change)\n")
	b.WriteString("Do NOT proceed until the user responds.\n")
	b.WriteString("</agent-instructions>")

	return b.String()
}
```

- [ ] **Step 4: Update FormatReviewResult to pass sessionID**

Change the call site in `FormatReviewResult`:

```go
if summary.Open > 0 {
	b.WriteString("\n\n")
	b.WriteString(buildAgentInstructions(findings, summary, sessionID))
}
```

Update the `buildAgentInstructions` signature to accept `sessionID string`.

- [ ] **Step 5: Fix existing formatter tests**

Update any tests that check agent-instructions content to match the new text.

- [ ] **Step 6: Run all formatter tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/formatter/ -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/formatter/xml.go internal/formatter/formatter_test.go
git commit -m "feat: agent-instructions require Claude Code to verify findings before presenting"
```

### Task 7: Update SKILL.md to include verification workflow

**Files:**
- Modify: `.claude/skills/xreview/SKILL.md`

- [ ] **Step 1: Rewrite Step 2.5 in SKILL.md**

Replace the current Step 2.5 with a two-phase process:

```markdown
## Step 2.5: Verify + Fix Plan Gate (MANDATORY)

<CRITICAL>
- You MUST independently verify EVERY finding before presenting to the user.
- Do NOT blindly copy Codex output. You are a capable code reviewer — USE your judgement.
- After verification, present only CONFIRMED findings as a fix plan.
- You MUST end the fix plan with AskUserQuestion. No exceptions.
- The xreview output includes `<agent-instructions>` after `</xreview-result>`. Follow them.
</CRITICAL>

Parse the XML output from Step 2.

If verdict is APPROVED (zero findings): tell the user "No issues found." Skip to Step 5.

### Phase 1: Verify Each Finding

For EACH finding in the XML output:

1. **Read the actual code** at the file:line referenced by the finding.
2. **Analyze validity** — does the issue actually exist?
   - For concurrency/lock findings: check lock scope (nested vs sequential locking),
     whether locks are actually held simultaneously, real contention scenarios.
   - For logic findings: trace the actual code path end-to-end.
   - For security findings: confirm untrusted input actually reaches the vulnerable code.
3. **Classify**:
   - **CONFIRMED**: the issue is real, you verified it in the code.
   - **SUSPECT**: you believe it may be a false positive.

For SUSPECT findings, challenge Codex before dropping them:

Run: `xreview review --session <session-id> --message "F-XXX appears to be a false positive: <your reasoning>. Please re-evaluate."`

Parse the response:
- If Codex agrees → drop the finding (don't present to user)
- If Codex provides valid counter-reasoning → mark as CONFIRMED
- If disagreement persists → present both perspectives to user with a note

### Phase 2: Build the Fix Plan (confirmed findings only)

For EACH confirmed finding, present:

1. **Header**: `### F-XXX: title (category/severity)` + `📍 file:line`
2. **Trigger**: the trigger condition — verified by your own code reading
3. **Impact**: what happens if exploited/triggered
4. **Cascade**: list every cascade impact — what else breaks
5. **Fix options**: ALL alternatives, mark which is recommended.
   Always add a final option: "Don't fix — risk: _consequence_"

Low severity findings may use a shorter format but MUST still include fix options.

### Get User Approval

After listing ALL confirmed findings, use AskUserQuestion:

```
Fix plan for N confirmed findings (M suspect findings dropped after Codex discussion). How to proceed?
  A. Execute all recommended fixes
  B. Only fix high severity, skip the rest
  C. I want to adjust (tell me which findings to change)
```

Do NOT proceed until user responds.
```

- [ ] **Step 2: Commit**

```bash
git add .claude/skills/xreview/SKILL.md
git commit -m "feat: SKILL.md Step 2.5 now requires Claude Code to verify findings independently"
```

### Task 8: Update SKILL.md Step 1 to guide Claude Code on review modes

**Files:**
- Modify: `.claude/skills/xreview/SKILL.md`

Now that Codex fetches code itself, Claude Code needs clear guidance on when to use `--files` vs `--git-uncommitted` and how to write effective `--context` for different scenarios.

- [ ] **Step 1: Rewrite Step 1 in SKILL.md**

Replace the current Step 1 with:

```markdown
## Step 1: Determine review targets and assemble context

Two review modes — pick the one that fits:

### Mode A: Review uncommitted changes (`--git-uncommitted`)
Use when reviewing what's about to be committed. Codex will run `git diff`
to see the changes itself.

### Mode B: Review specific files (`--files`)
Use when:
- Reviewing a single file's quality (not tied to a git change)
- Reviewing a flow/feature that spans multiple files
- The user specifies which files to look at

Codex will read the files directly — no git diff involved.

### Assembling `--context`

The context string is critical — it tells Codex **what to focus on**.

For **git-uncommitted** (change-focused):
```
--context "【變更類型】feature | refactor | bugfix
【描述】簡述做了什麼
【預期行為】這段 code 應該達成什麼效果"
```

For **files** (review-focused), describe the review focus:
```
--context "【Review 焦點】Review the CMS push event flow:
enqueue → EventQueue.push() → purge logic → SendQueue routing.
Focus on concurrency safety and lock correctness across these files.
【預期行為】cache 和 ordered 路徑完全獨立不互鎖"
```

For **files** (single file quality):
```
--context "【Review 焦點】General quality review of event_queue.cpp.
Look for bugs, race conditions, error handling issues."
```

The better the context, the better Codex's review. Be specific about
the flow direction, expected behavior, and areas of concern.
```

- [ ] **Step 2: Commit**

```bash
git add .claude/skills/xreview/SKILL.md
git commit -m "docs: update SKILL.md Step 1 with review mode guidance and context examples"
```

### Task 9: Run full test suite and manual smoke test

- [ ] **Step 1: Run all tests**

Run: `cd /home/davidleitw/xreview && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Build binary**

Run: `cd /home/davidleitw/xreview && go build ./cmd/xreview/`
Expected: SUCCESS

- [ ] **Step 3: Install updated binary**

Run: `cd /home/davidleitw/xreview && go install ./cmd/xreview/`

- [ ] **Step 4: Manual smoke test**

Run a review on a small file to verify:
1. The prompt sent to Codex contains fetch instructions (not file content)
2. Codex can successfully run the git command and produce findings
3. Agent-instructions contain the verification phase

```bash
xreview review --files internal/formatter/xml.go --context "test: verify codex self-fetch works" 2>&1 | head -50
```

- [ ] **Step 5: Final commit if any fixups**

---

## Summary of Changes

| File | Change |
|------|--------|
| `internal/prompt/templates.go` | Rewrite both templates: remove embedded diff/files, add FetchMethod instruction |
| `internal/prompt/builder.go` | Update `FirstRoundInput` and `ResumeInput` structs: replace Diff with FetchMethod |
| `internal/prompt/builder_test.go` | Update all tests for new struct fields and template content |
| `internal/reviewer/single.go` | Add `buildFetchMethod()`, `buildFileListSummary()`. Update Review()/Verify() to use them. Remove `formatFilesForPrompt()` |
| `internal/reviewer/single_test.go` | Update tests for new prompt building logic |
| `internal/formatter/xml.go` | Rewrite `buildAgentInstructions()` with verification phase + session ID |
| `internal/formatter/formatter_test.go` | Update tests for new agent-instructions content |
| `.claude/skills/xreview/SKILL.md` | Rewrite Step 2.5 with verify-then-present workflow |

## Out of Scope

- Collector changes (still reads file content for line counts — negligible cost)
- CLI flag changes (no new flags needed)
- Schema changes (Codex output format unchanged)
- Session storage changes
- Report/clean/preflight commands
