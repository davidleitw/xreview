# xreview Stub Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fill in all stub implementations with real logic and unit tests. No integration/E2E tests — unit tests only.

**Architecture:** Bottom-up: leaf implementations first (collector, formatter, codex runner), then orchestration (reviewer), then CLI wiring.

**Tech Stack:** Go 1.22+, `os/exec`, `encoding/xml`, `text/template`, `testing`

---

### Task 1: Collector — file reading + git integration

**Files:**
- Modify: `internal/collector/collector.go`
- Modify: `internal/collector/git.go`
- Create: `internal/collector/collector_test.go`
- Create: `internal/collector/git_test.go`

**Implementation:**

`collector.go` — `Collect()`:
- If mode is "git-uncommitted": call `GitUncommittedFiles(workdir)` to get file list
- If mode is "files": expand directories (walk), apply ignore patterns from config
- Read each file, populate `FileContent{Path, Content, Lines}`

`git.go` — `GitUncommittedFiles()`:
- Run `git diff --name-only` + `git diff --cached --name-only`
- Merge and deduplicate results
- Return relative paths

**Tests:**
- `collector_test.go`: create temp dir with files, test Collect in "files" mode
- `git_test.go`: test GitUncommittedFiles with a temp git repo

---

### Task 2: Formatter — XML output for all commands

**Files:**
- Modify: `internal/formatter/error.go`
- Modify: `internal/formatter/xml.go`
- Create: `internal/formatter/formatter_test.go`

**Implementation:**

`error.go` — `FormatError(action, code, message)`:
- Produce `<xreview-result status="error" action="..."><error code="...">message</error></xreview-result>`

`xml.go` — all Format* functions:
- `FormatReviewResult(sessionID, round, verdict string, findings []Finding, summary FindingSummary)` → XML with findings
- `FormatPreflightResult(checks []PreflightCheck)` → XML with check results
- `FormatVersionResult(current, latest string, outdated bool)` → XML
- `FormatReportResult(sessionID, reportPath string)` → XML
- `FormatCleanResult(sessionID string)` → XML

**Tests:**
- Test each formatter function produces valid XML with expected attributes

---

### Task 3: Codex Runner — subprocess execution

**Files:**
- Modify: `internal/codex/runner.go`
- Create: `internal/codex/runner_test.go`

**Implementation:**

`runner.go` — `Exec()`:
- Build command: `codex exec -m <model> --output-schema <schema-path> --skip-git-repo-check`
- If ResumeSessionID != "": add `-r <id>`
- Pass prompt via stdin
- Capture stdout, stderr
- Extract codex session ID from stderr
- Return ExecResult with duration

**Tests:**
- Test command building logic (extract the command assembly into a testable helper)
- Test timeout handling

---

### Task 4: Prompt Builder — FormatFindingsForPrompt

**Files:**
- Modify: `internal/prompt/builder.go`
- Create: `internal/prompt/builder_test.go`

**Implementation:**

`builder.go` — `FormatFindingsForPrompt()`:
- Format each finding as readable text: `[ID] (severity/category) file:line — description`
- Include status for verify rounds

**Tests:**
- Test BuildFirstRound with sample input
- Test BuildResume with sample input
- Test FormatFindingsForPrompt with various finding lists

---

### Task 5: Reviewer — SingleReviewer orchestration

**Files:**
- Modify: `internal/reviewer/single.go`
- Create: `internal/reviewer/single_test.go`

**Implementation:**

`single.go` — `Review()`:
1. Create session via manager
2. Collect files via collector
3. Build prompt via builder (first round)
4. Write temp schema
5. Call codex via runner
6. Parse response
7. Update session with findings
8. Return ReviewResult

`single.go` — `Verify()`:
1. Load session
2. Collect files (if needed)
3. Build resume prompt with message + current findings
4. Call codex (resume or fresh based on ShouldResume)
5. Parse response, merge findings
6. Update session
7. Return VerifyResult

**Tests:**
- Mock all interfaces (Runner, Builder, Parser, Manager, Collector)
- Test Review happy path
- Test Verify happy path
- Test error propagation

---

### Task 6: CLI Commands — wire everything together

**Files:**
- Modify: `cmd/xreview/cmd_preflight.go`
- Modify: `cmd/xreview/cmd_review.go`
- Modify: `cmd/xreview/cmd_report.go`
- Modify: `cmd/xreview/cmd_clean.go`
- Modify: `cmd/xreview/cmd_selfupdate.go`

**Implementation:**

`cmd_preflight.go`:
- Check `exec.LookPath("codex")`
- Run `codex --version` to verify auth
- Run simple test prompt to verify responsiveness
- Output via formatter

`cmd_review.go`:
- Build dependencies (config, collector, prompt builder, parser, runner, session manager, reviewer)
- New review: call reviewer.Review()
- Resume: call reviewer.Verify()
- Output via formatter

`cmd_report.go`:
- Load session, format findings as report, write to file
- Output via formatter

`cmd_clean.go`:
- Delete session via manager
- Output via formatter

`cmd_selfupdate.go`:
- `go install github.com/davidleitw/xreview@latest`
- Output via formatter

---
