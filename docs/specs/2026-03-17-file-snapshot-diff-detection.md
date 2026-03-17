# File Snapshot & Change Detection for Multi-Round Review

**Date**: 2026-03-17
**Status**: Draft

## Problem

In multi-round reviews, Codex resumes its session but may not re-read files that
were modified between rounds. It relies on cached context from round 1, so even
after the developer fixes code, Codex evaluates findings against stale content.

This affects both review modes:
- `git-uncommitted`: Codex has the old `git diff` output in its conversation history
- `files`: Codex has the old file content in its conversation history

The root cause is not a Codex limitation — Codex *can* read files when resumed.
The problem is that its conversation history already contains the previous content,
so it may skip re-reading unless explicitly told which files changed.

## Solution

Add file snapshot tracking to xreview sessions. At the end of each round, record
a checksum for every target file. At the start of the next round, compare checksums
to detect which files changed, then include a **changed files list** in the resume
prompt so Codex knows exactly what to re-read.

### Design Principles

1. **xreview owns the state** — change detection is done by xreview, not delegated
   to Codex. This works regardless of which reviewer backend is used.
2. **Prompt carries signals, not content** — the resume prompt tells Codex *which*
   files changed, not *what* changed. Codex reads the files itself. This avoids
   bloating the prompt with file contents.
3. **Codex session resume is preserved** — we keep the `codex exec resume` mechanism
   so Codex retains its conversation memory (reasoning context, codebase understanding).

## Data Model Change

Add to `Session`:

```go
type FileSnapshot struct {
    Path     string `json:"path"`      // relative to workdir
    Checksum string `json:"checksum"`  // SHA-256 hex
}

type Session struct {
    // ...existing fields...
    FileSnapshots []FileSnapshot `json:"file_snapshots,omitempty"`
}
```

Paths are stored relative to workdir (consistent with `sess.Targets`).

## Checksum Implementation

```go
func checksumFile(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }
    return hex.EncodeToString(h.Sum(nil)), nil
}
```

SHA-256, not mtime. Mtime is unreliable across git checkout, CI, and cross-machine
scenarios. Checksum is the only way to confirm content actually changed.

## Snapshot Scope by Mode

| Mode | What gets snapshotted |
|------|----------------------|
| `git-uncommitted` | Files from `git diff --name-only` + `git diff --cached --name-only` + untracked files |
| `files` | All files in `sess.Targets` (expanded from directories) |

## Flow

### Round 1 (Review)

1. Run review as normal
2. After Codex responds, snapshot all target files → store in `sess.FileSnapshots`
3. Save session

### Round 2+ (Verify)

1. Load session (includes `FileSnapshots` from previous round)
2. Snapshot current files → compare with stored snapshots
3. Produce change list: `modified`, `added`, `deleted`
4. Pass change list to prompt builder → inject into `ResumeTemplate`
5. Run Codex with resume
6. After Codex responds, update `sess.FileSnapshots` with new snapshot
7. Save session

## Prompt Change

Add `ChangedFiles` to `ResumeInput`:

```go
type FileChange struct {
    Path   string // relative path
    Status string // "modified", "added", "deleted"
}

type ResumeInput struct {
    // ...existing fields...
    ChangedFiles []FileChange
}
```

Add a section to `ResumeTemplate`:

```
{{if .ChangedFiles}}
===== FILES CHANGED SINCE LAST ROUND =====
The following files have been modified since your last review.
You MUST re-read these files before evaluating the findings.

{{range .ChangedFiles}}- [{{.Status}}] {{.Path}}
{{end}}
===== END =====
{{else}}
No files have changed since your last review.
Evaluate the findings based on the developer's message and your existing knowledge of the code.
{{end}}
```

The existing `FetchMethod` section remains as-is — it tells Codex *how* to read
files. The new section tells it *which* files to re-read.

## Files to Modify

| File | Change |
|------|--------|
| `internal/session/types.go` | Add `FileSnapshot` struct, add `FileSnapshots` field to `Session` |
| `internal/collector/collector.go` | Add `Snapshot(targets, mode, workdir) ([]FileSnapshot, error)` method |
| `internal/reviewer/single.go` | Call `Snapshot()` after round 1; compare snapshots in `Verify()`; pass `ChangedFiles` to prompt builder |
| `internal/prompt/builder.go` | Add `ChangedFiles` to `ResumeInput`; add `FileChange` type |
| `internal/prompt/templates.go` | Add changed files section to `ResumeTemplate` |

No changes to: skill files, CLI flags, Codex invocation, formatter, parser.

## What This Does NOT Change

- **Codex session resume** — still used, Codex keeps its conversation memory
- **Prompt size** — only file names added, not content
- **CLI interface** — no new flags
- **Skill behavior** — skill is unaware of this; it's internal to xreview binary

## Future Compatibility

This change is additive and does not conflict with planned features:

- **Second opinion**: independent reviewer session, doesn't use resume, unaffected
- **Review plan**: single-round, no multi-round needed, unaffected
- **Auto-fix mode**: benefits from accurate change detection between fix rounds
- **Replacing Codex backend**: `FileSnapshots` is backend-agnostic — any reviewer
  benefits from being told which files changed
- **Multi-reviewer sessions**: snapshots are session-level, shared across reviewers
