# Three Bugfix Design: Error Classification, Resume Files, Resume JSON

Date: 2026-03-11

## Problem Statement

Three related issues discovered during real-world usage of `xreview review`:

1. **Misclassified error**: `classifyReviewError()` uses broad string matching that false-positives on `SESSION_NOT_FOUND` when codex fails for unrelated reasons (codex stderr banner contains "session id: ...").
2. **UX gap**: `--session` + `--files` is blocked by validation, but users naturally want to add files to an existing review session.
3. **Resume parse failure**: `codex exec resume` does not support `--output-schema`, and `ResumeTemplate` has no JSON format instruction, so codex returns free-form text that `ExtractJSON()` cannot parse.

## Evidence

Tested with `/tmp/xreview-test/test_codex_resume_schema.py`:

- `codex exec resume --output-schema` exits with code 2: `unexpected argument '--output-schema'`
- `codex exec resume` without schema but with prompt guidance returns valid JSON (Step 3 succeeded)
- Codex stderr banner always contains `session id: <uuid>`, polluting error string matching

## Design

### Fix 1: Tighten `classifyReviewError()` string matching

**File**: `cmd/xreview/cmd_review.go`

**Current** (line 149):
```go
case strings.Contains(msg, "session") && strings.Contains(msg, "not found"):
```

**Change to**:
```go
case strings.Contains(msg, "session") && strings.Contains(msg, "\" not found"):
```

This matches the exact format from `session/manager.go`: `session "xr-..." not found`, while excluding codex stderr that contains `session id: <uuid>`.

### Fix 2: Allow `--session` + `--files` for resume with extra files

**Files**: `cmd/xreview/cmd_review.go`, `internal/reviewer/single.go`

Changes:
1. **Remove the `PreRunE` mutual exclusion** between `--session` and `--files`/`--git-uncommitted`.
2. **Add `ExtraTargets` and `ExtraTargetMode` fields** to `VerifyRequest`.
3. **In `Verify()`**: if `ExtraTargets` is provided, collect those files and append their content to the resume prompt under a new `===== ADDITIONAL FILES =====` section.
4. **In `cmd_review.go` `RunE`**: when `sessionID != ""` and files/git-uncommitted are also set, pass them as `ExtraTargets`.

The `ResumeInput` struct gets an optional `AdditionalFiles` field. The template conditionally renders it.

### Fix 3: Add JSON format instruction to `ResumeTemplate`

**File**: `internal/prompt/templates.go`

Append to `ResumeTemplate`:

```
Respond with ONLY a JSON object in this exact format (no markdown fences, no extra text):
{
  "verdict": "APPROVED" or "REVISE",
  "summary": "brief summary",
  "findings": [
    {
      "id": "F-001",
      "severity": "high|medium|low",
      "category": "security|logic|performance|error-handling",
      "file": "path/to/file.cpp",
      "line": 42,
      "description": "what is wrong",
      "suggestion": "how to fix",
      "code_snippet": "relevant code",
      "status": "open|fixed|dismissed|reopened",
      "verification_note": "verification details"
    }
  ]
}
```

Additionally, harden `ExtractJSON()` as a safety net: if the output doesn't start with `{` or code fences, scan for the first `{` that could be the start of a JSON object and try to extract from there.

## Files to Change

| File | Change |
|------|--------|
| `cmd/xreview/cmd_review.go` | Fix 1: tighten string match. Fix 2: relax PreRunE, pass extra targets to Verify |
| `internal/reviewer/single.go` | Fix 2: accept ExtraTargets in VerifyRequest, collect and append to prompt |
| `internal/reviewer/types.go` | Fix 2: add ExtraTargets/ExtraTargetMode to VerifyRequest |
| `internal/prompt/templates.go` | Fix 3: add JSON format instruction to ResumeTemplate |
| `internal/prompt/builder.go` | Fix 2: add AdditionalFiles field to ResumeInput |
| `internal/parser/extract.go` | Fix 3: add fallback JSON extraction (find first `{`) |
| Tests | Update `runner_test.go`, add `extract_test.go` cases |

## Out of Scope

- Changing to sentinel errors / custom error types (overkill for now)
- Changing `FirstRoundTemplate` (already works with `--output-schema`)
- Changing codex runner's resume args (confirmed resume doesn't support `--output-schema`)
