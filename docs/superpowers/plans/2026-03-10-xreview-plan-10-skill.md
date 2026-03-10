# Plan 10: Claude Code Skill + Finalization

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the Claude Code skill files (SKILL.md + reference.md) and finalize the project with .gitignore.

**Architecture:** Project-level skill at `.claude/skills/xreview/`. SKILL.md defines the workflow, reference.md documents the XML schema.

**Tech Stack:** Markdown, YAML frontmatter

**Depends on:** Plans 1-9 (CLI must be fully functional)

---

## Chunk 1: Skill Files + Project Finalization

### File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `.claude/skills/xreview/SKILL.md` | Claude Code skill definition |
| Create | `.claude/skills/xreview/reference.md` | XML schema reference |
| Create | `.gitignore` | Ignore build artifacts and session data |

---

### Task 10.1: SKILL.md

**Files:**
- Create: `.claude/skills/xreview/SKILL.md`

- [ ] **Step 1: Write SKILL.md**

Create `.claude/skills/xreview/SKILL.md` with the exact content from the design spec Section 8:

```yaml
---
name: xreview
description: >
  AI-powered code review using Codex. Triggers after completing a plan or
  milestone to review changed files for bugs, security issues, and logic errors.
  Manages the full review lifecycle: discover, fix, verify, report.
allowed-tools: Bash(xreview *), Bash(go install *), Bash(which *), AskUserQuestion, Read
argument-hint: [files-or-uncommitted]
---

# xreview - Agent-Native Code Review

## Step 0: Ensure xreview is installed

1. Check if xreview exists:
   Run: `which xreview`

2. If NOT found:
   a. Check if Go is available: `which go`
   b. If Go is available:
      - Ask the user: "xreview is not installed. Install it now? (y/n)"
      - If yes: run `go install github.com/davidleitw/xreview@latest`
      - Verify: run `xreview version`
      - If install fails, show the error and stop.
   c. If Go is NOT available:
      - Tell the user: "xreview requires Go to install. Please install Go,
        or download the binary from https://github.com/davidleitw/xreview/releases"
      - Stop.

3. If found, check version:
   Run: `xreview version`
   - Parse the XML output for the `outdated` attribute.
   - If outdated="true":
     Ask user: "xreview {current} is installed but {latest} is available. Update? (y/n)"
     If yes: run `xreview self-update`

## Step 1: Ask user if they want a review

Ask the user: "Code review? (y/n)"
If no, stop. Do not proceed with any review steps.

## Step 2: Preflight check

Run: `xreview preflight`

Parse the XML output:
- If status="success": proceed to Step 3.
- If status="error": show the user the error message from the <error> tag.
  The error message is written for you to understand. Relay it to the user
  in natural language and suggest how to fix it. Stop.

## Step 3: Determine review targets

Based on the current task context, determine which files to review:
- If you just completed a plan with specific files changed, use --files with those paths.
- If unsure which files changed, use --git-uncommitted.
- Write a brief --context describing what was implemented.

## Step 4: Run review

Run: `xreview review --files <file1,file2,...> --context "<description>"`
 or: `xreview review --git-uncommitted --context "<description>"`

## Step 5: Present findings

Parse the XML output. For each <finding>, present it to the user in plain language:

"Found {N} issues:
 - {SEVERITY} {file}:{line} - {description}
 - ..."

If verdict is APPROVED (zero findings): tell the user "No issues found." Skip to Step 8.

Ask the user: "Fix these? (y/n)"

## Step 6: Address findings (multi-round)

For each finding, YOU (Claude Code) decide one of:
a) Fix it yourself — read the file, understand the suggestion, apply the fix.
b) Disagree with codex — include your reasoning in the --message for the next round.
   Codex will re-evaluate. This enables multi-round discussion between you and codex.
c) Skip it — tell the user why you think it's not worth fixing.

Keep track of what you fixed, disagreed with, and skipped.

## Step 7: Verify fixes

After addressing findings, run:
`xreview review --session <session-id> --message "<what you fixed, disagreed with, or skipped>"`

Parse the XML output:
- Show updated finding statuses to the user.
- If codex and you still disagree on something, you can continue the discussion
  in the next --message. This is a conversation between two agents.
- If there are still open findings, go back to Step 5.
- If all findings are resolved (fixed + dismissed), proceed to Step 8.

Limit to a maximum of 5 rounds. If still open findings after 5 rounds,
inform the user and proceed to Step 8.

## Step 8: Finalize

Run: `xreview report --session <session-id>`

Tell the user: "Review complete. Report saved to {report-path}."
Provide a brief summary of the final finding statuses.

Ask the user (AskUserQuestion): "Clean up the review session? (y/n)"
If yes: run `xreview clean --session <session-id>` to remove session data.

## Important notes

- ALWAYS present findings in plain language, NOT raw XML.
- The <error> messages from xreview are written for you (an AI agent). Use them
  to understand what went wrong and explain it to the user naturally.
- Do NOT read or write .xreview/ directory files directly. Use only xreview CLI commands.
- The session ID is in the XML output's session attribute. Track it for resume calls.
- If any xreview command fails, show the error to the user and ask how to proceed.
- Preflight only runs once per session. If a later command fails with a codex error,
  the error message will tell you what happened — no need to re-run preflight.
- This is a MULTI-ROUND REVIEW. You and codex may disagree. Use --message to have a
  natural conversation. Codex is smart enough to reconsider when given good reasoning.

## XML Schema Reference

See [reference.md](reference.md) for the complete XML schema documentation.
```

- [ ] **Step 2: Commit**

```bash
git add .claude/skills/xreview/SKILL.md
git commit -m "feat: add Claude Code skill SKILL.md"
```

---

### Task 10.2: reference.md

**Files:**
- Create: `.claude/skills/xreview/reference.md`

- [ ] **Step 1: Write reference.md**

Create `.claude/skills/xreview/reference.md`:

```markdown
# xreview XML Schema Reference

This document describes the XML output format of xreview CLI commands.
Claude Code skill uses this to parse xreview results.

## Envelope

All output is wrapped in:

```
<xreview-result status="success|error" action="..." session="..." round="N">
```

## Elements

### <finding>
Attributes: id, severity (high|medium|low), category, status (open|fixed|dismissed|reopened)
Optional attributes: comparison (recurring|new) — only in full-rescan mode
Children: <location>, <description>, <suggestion>, <code-snippet>, <verification>

### <location>
Attributes: file (path), line (number)

### <summary>
Attributes: total, open, fixed, dismissed
Additional in report: rounds, total-findings

### <error>
Attributes: code (see error code table)
Content: human-readable error description with suggested action

### <checks> (preflight only)
Children: <check name="..." passed="true|false" detail="..." />

### <version> (version only)
Attributes: current, latest, outdated (true|false), update-command

### <report> (report only)
Attributes: path (file path to generated report)

### <resolved-from-previous> (full-rescan only)
Children: <resolved id="..." note="..." />

### <cleaned> (clean only)
Attributes: session

### <update> (self-update only)
Attributes: from, to, already-latest

## Error Codes

| Code | Meaning |
|------|---------|
| CODEX_NOT_FOUND | codex binary not in PATH |
| CODEX_NOT_AUTHENTICATED | codex not logged in |
| CODEX_UNRESPONSIVE | codex did not respond to test prompt |
| CODEX_TIMEOUT | codex exceeded timeout |
| CODEX_ERROR | codex exited with non-zero code |
| PARSE_FAILURE | could not parse codex output |
| SESSION_NOT_FOUND | session ID does not exist |
| NO_TARGETS | no files to review |
| FILE_NOT_FOUND | specified file does not exist |
| NOT_GIT_REPO | --git-uncommitted used outside git repo |
| INVALID_FLAGS | invalid flag combination |
| UPDATE_FAILED | self-update failed |
| VERSION_CHECK_FAILED | cannot reach GitHub API for version check |
```

- [ ] **Step 2: Commit**

```bash
git add .claude/skills/xreview/reference.md
git commit -m "feat: add XML schema reference for Claude Code skill"
```

---

### Task 10.3: .gitignore

**Files:**
- Create: `.gitignore`

- [ ] **Step 1: Write .gitignore**

Create `.gitignore`:

```
# Build artifacts
bin/
dist/

# Session data (generated at runtime)
.xreview/sessions/

# Go
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db
```

- [ ] **Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: add .gitignore"
```

---

### Task 10.4: Sample Session Fixture for Tests

**Files:**
- Create: `test/fixtures/sessions/sample-session/session.json`
- Create: `test/fixtures/sessions/sample-session/findings.json`
- Create: `test/fixtures/sessions/sample-session/rounds/round-001.json`

- [ ] **Step 1: Create sample session fixture**

Create `test/fixtures/sessions/sample-session/session.json`:

```json
{
  "session_id": "xr-20260310-a1b2c3",
  "xreview_version": "0.1.0",
  "created_at": "2026-03-10T14:30:00Z",
  "updated_at": "2026-03-10T14:45:00Z",
  "status": "in_review",
  "current_round": 1,
  "codex_session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "codex_model": "o3",
  "context": "Implemented JWT authentication system",
  "targets": ["src/auth.go", "src/middleware.go"],
  "target_mode": "files",
  "config": {
    "timeout": 180
  }
}
```

Create `test/fixtures/sessions/sample-session/findings.json`:

```json
{
  "last_updated_round": 1,
  "findings": [
    {
      "id": "F001",
      "severity": "high",
      "category": "security",
      "status": "open",
      "file": "src/auth.go",
      "line": 42,
      "description": "JWT token is not checked for expiration.",
      "suggestion": "Add exp claim validation.",
      "code_snippet": "token, err := jwt.Parse(rawToken, keyFunc)",
      "first_seen_round": 1,
      "last_updated_round": 1,
      "history": [
        {"round": 1, "status": "open", "note": "initial finding"}
      ]
    }
  ],
  "summary": {
    "total": 1,
    "open": 1,
    "fixed": 0,
    "dismissed": 0
  }
}
```

Create `test/fixtures/sessions/sample-session/rounds/round-001.json`:

```json
{
  "round": 1,
  "timestamp": "2026-03-10T14:30:05Z",
  "action": "review",
  "codex_session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "codex_resumed": false,
  "full_rescan": false,
  "user_message": "",
  "targets_snapshot": ["src/auth.go", "src/middleware.go"],
  "findings_before": {"total": 0, "open": 0, "fixed": 0, "dismissed": 0},
  "findings_after": {"total": 1, "open": 1, "fixed": 0, "dismissed": 0},
  "raw_stdout_path": "raw/round-001-codex-stdout.txt",
  "raw_stderr_path": "raw/round-001-codex-stderr.txt",
  "duration_ms": 8500
}
```

- [ ] **Step 2: Commit**

```bash
mkdir -p test/fixtures/sessions/sample-session/rounds
git add test/fixtures/sessions/
git commit -m "test: add sample session fixture data"
```

---

### Task 10.5: Final Verification

- [ ] **Step 1: Run all unit tests**

```bash
cd /home/davidleitw/xreview && make test
```

Expected: All PASS

- [ ] **Step 2: Run integration tests**

```bash
cd /home/davidleitw/xreview && make test-integration
```

Expected: All PASS

- [ ] **Step 3: Build and test help**

```bash
cd /home/davidleitw/xreview && make build && ./bin/xreview --help
```

Expected: All 6 commands listed

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "chore: finalize xreview v0.1.0 — all tests passing"
```
