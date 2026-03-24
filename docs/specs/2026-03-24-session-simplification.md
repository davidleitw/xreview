# Session Simplification & Report Removal

**Date:** 2026-03-24
**Status:** Draft

## Motivation

Session files in `.xreview/sessions/` are implementation details of the multi-round review flow. They have no value to the user after a review ends. The `report` command and `write-report` skill generate artifacts (JSON/markdown reports) that nobody reads — git history and conversation context already capture what was fixed, dismissed, or left open.

This spec simplifies xreview by:
1. Moving sessions to `/tmp` (ephemeral by nature)
2. Removing the report command and write-report skill
3. Having Claude Code produce a detailed verbal summary instead

## Changes

### 1. Session storage: `.xreview/sessions/` -> `/tmp/xreview/sessions/`

**Current:** `<workdir>/.xreview/sessions/<session-id>/session.json`
**New:** `/tmp/xreview/sessions/<session-id>/session.json`

- Session ID format unchanged: `xr-YYYYMMDD-<hex>`
- `config.SessionsDir()` changes from `filepath.Join(workdir, ".xreview", "sessions")` to `/tmp/xreview/sessions`
- No more session data polluting the working directory
- If a Claude Code session breaks mid-review, the user simply re-runs the review. Stale temp sessions are harmless and cleaned by OS or `xreview clean`.

**Permissions:** Session directories MUST be created with `0o700` (user-only) to prevent other users on shared systems from reading or deleting sessions.

**Interface change:** `NewManager` drops the `workdir` parameter entirely — `NewManager() Manager`. The sessions path is hardcoded to `/tmp/xreview/sessions`. All call sites update accordingly.

**`config.SessionsDir()`** becomes a no-arg function returning `/tmp/xreview/sessions`. Remove the `workdir` parameter and `SessionsDirName` constant.

### 2. Remove `report` command

Delete `cmd/xreview/cmd_report.go` entirely. Remove registration from `main.go`.

The `report` struct and `generateReport()` function are only used by this command — delete them too.

### 3. Remove `write-report` skill

Delete `skills/write-report/SKILL.md`.

### 4. Update `clean` command

**Current:** Deletes a single session from `.xreview/sessions/` by ID.
**New:** Two modes:
- `xreview clean --session <id>` — deletes `/tmp/xreview/sessions/<id>/`
- `xreview clean --all` — deletes `/tmp/xreview/sessions/` entirely

The command no longer needs `--workdir` for session operations.

### 5. Update review skill Step 5 (finalize)

**Current:** Calls write-report skill, which runs `xreview report`, generates markdown, saves file, runs `xreview clean`.

**New Step 5:**

```
## Step 5: Finalize

Produce a detailed summary in the conversation. Cover:

1. **Overview** — what was reviewed, how many rounds, overall verdict
2. **Findings resolved** — for each fixed finding: what the issue was, what was changed
3. **Findings dismissed** — for each: why it was dismissed (false positive, user decision, etc.)
4. **Findings still open** — for each: why it remains open, recommended next steps
5. **Session cleanup** — run `xreview clean --session <session-id>`

This summary IS the report. Do not save it to a file.
```

### 6. Remove `.xreview/reports/` directory concern

The `report` command was the only thing writing to `.xreview/reports/`. With it gone, this directory is never created. No cleanup needed.

## Testing

`manager_test.go` currently uses `t.TempDir()` as workdir. After the change, `SessionsDir()` returns a fixed `/tmp/xreview/sessions` path, which would cause test isolation issues.

**Fix:** Add `SessionsDirOverride` — a package-level variable (default empty). When set, `SessionsDir()` returns the override. Tests set it to `t.TempDir()` via `TestMain` or per-test setup. Production code never touches it.

## Migration

Users who upgrade from a version that stored sessions in `.xreview/sessions/` will have stale session directories. No migration needed — these are dead files. Users can `rm -rf .xreview/sessions .xreview/reports` manually if they care. The `.xreview/` directory itself stays (it still holds `config.json`).

## Files affected

| File | Action |
|------|--------|
| `internal/config/config.go` | `SessionsDir()` no-arg, returns `/tmp/xreview/sessions`, add `SessionsDirOverride` for tests, remove `SessionsDirName` |
| `internal/session/manager.go` | `NewManager()` no-arg, uses `config.SessionsDir()`, create dirs with `0o700` |
| `internal/session/manager_test.go` | Use `SessionsDirOverride` for test isolation |
| `cmd/xreview/cmd_report.go` | Delete |
| `cmd/xreview/cmd_clean.go` | Add `--all` flag, point to `/tmp` path, remove `--workdir` dependency |
| `cmd/xreview/cmd_review.go` | Update `NewManager()` call (drop workdir arg) |
| `cmd/xreview/main.go` | Remove `newReportCmd()` registration |
| `internal/reviewer/single.go` | Update `NewManager()` call (drop workdir arg) |
| `skills/review/SKILL.md` | Rewrite Step 5 |
| `skills/write-report/SKILL.md` | Delete |
| `README.md` | Remove `report` command from CLI reference, update `clean` docs |

## What stays the same

- Session ID format (`xr-YYYYMMDD-hex`)
- Session JSON schema (types.go unchanged)
- Multi-round flow (resume via `--session`)
- All other commands (`preflight`, `review`, `version`, `self-update`)
- `.xreview/config.json` stays in workdir (project-level config is still useful)
