# Write-Report Skill & Review Workflow Improvements

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `write-report` skill that generates human-readable markdown reports from xreview sessions, and improve the review skill's context guidance.

**Architecture:** Pure skill-layer changes — no Go code modifications. A new `write-report` skill defines the report markdown layout and instructs Claude Code to combine `xreview report` structured data with conversation context. The existing review skill's Step 5 delegates to write-report, and Step 1 gets richer `--context` guidance.

**Tech Stack:** Claude Code skill markdown files only.

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `.claude/skills/xreview/write-report.md` | Create | New skill: report layout, generation instructions |
| `.claude/skills/xreview/SKILL.md` | Modify | Step 5 → invoke write-report; Step 1 → richer context guidance |

---

## Chunk 1: All Changes

### Task 1: Create `write-report` skill

**Files:**
- Create: `.claude/skills/xreview/write-report.md`

- [ ] **Step 1: Create the write-report skill file**

Create `.claude/skills/xreview/write-report.md` with the following content:

```markdown
---
name: write-report
description: >
  Generates a human-readable markdown review report. Use this after completing
  a code review session (invoked automatically by the xreview skill at Step 5,
  or manually via /write-report). Combines xreview session data with your
  conversation context to produce a comprehensive report.
allowed-tools: Bash(xreview *), Read, Write
argument-hint: <session-id>
---

# write-report — Generate Review Report

## Step 1: Get session data

Run: `xreview report --session <session-id>`

Parse the XML output to get:
- Session ID, round count
- Finding list with statuses (open/fixed/dismissed)
- Summary counts (total, open, fixed, dismissed)
- Report file path (the JSON report xreview saved)

If the command fails, show the error to the user and stop.

## Step 2: Generate markdown report

Combine the structured session data with what you know from the conversation:
- **Verification results** — which findings you confirmed/suspected, and why
- **User decisions** — what the user chose to fix, skip, or adjust
- **Fix details** — what code changes you made for each finding
- **Dismissal reasons** — why certain findings were not fixed

Write the report using this layout:

~~~
# Code Review Report

**Date:** YYYY-MM-DD
**Session:** <session-id>
**Reviewed:** <list of files or "uncommitted changes">
**Rounds:** <number of review rounds>

## Context

<The --context that was provided for this review. Include the motivation,
what was changed, and what the review was focusing on.>

## Summary

| Status | Count |
|--------|-------|
| Total findings | N |
| Fixed | N |
| Dismissed | N |
| Open | N |

## Findings

### F-XXX: <title> — <severity>/<category> ✅ Fixed

**Location:** `file.go:42`
**Issue:** <description of the problem>
**Fix applied:** <what was changed and why this approach was chosen>

### F-XXX: <title> — <severity>/<category> ⏭️ Dismissed

**Location:** `file.go:10`
**Issue:** <description>
**Reason:** <why this was dismissed — user decision or false positive>

### F-XXX: <title> — <severity>/<category> ⚠️ Open

**Location:** `file.go:99`
**Issue:** <description>
**Note:** <why this remains open — e.g. deferred to next PR, needs more context>

## Open Items

<If any findings remain open, summarize what's left and any recommended next steps.
If all findings are resolved, write "All findings have been addressed.">
~~~

**Layout rules:**
- Group findings by status: Fixed first, then Dismissed, then Open
- For Fixed findings: always describe what was actually changed, not just the suggestion
- For Dismissed findings: always include the reason (user decision, false positive, etc.)
- Use the emoji status markers: ✅ Fixed, ⏭️ Dismissed, ⚠️ Open
- Keep descriptions concise — the report is a record, not a tutorial

## Step 3: Save and present

1. **Save the full report** to `.xreview/reports/<session-id>.md`
   using the Write tool.

2. **Print a summary in the conversation:**

```
Review complete. Report saved to .xreview/reports/<session-id>.md

Summary: N findings total — X fixed, Y dismissed, Z open.
```

3. **Ask about cleanup** (AskUserQuestion):
   "Clean up the review session data? (y/n)"
   If yes: run `xreview clean --session <session-id>`
```

- [ ] **Step 2: Verify the skill file exists and has correct frontmatter**

Run: `cat .claude/skills/xreview/write-report.md | head -6`

Expected: the frontmatter with `name: write-report` and `allowed-tools`.

---

### Task 2: Modify review skill SKILL.md — Step 5

**Files:**
- Modify: `.claude/skills/xreview/SKILL.md:194-203`

- [ ] **Step 1: Replace Step 5 content**

In `.claude/skills/xreview/SKILL.md`, replace the current Step 5 (lines 194-203):

```markdown
## Step 5: Finalize

Run: `xreview report --session <session-id>`

Tell the user: "Review complete. Report saved to {report-path}."
Provide a brief summary of the final finding statuses.

Ask the user (AskUserQuestion): "Clean up the review session? (y/n)"
If yes: run `xreview clean --session <session-id>` to remove session data.
```

With:

```markdown
## Step 5: Finalize

Invoke the `write-report` skill with the session ID to generate the final report.

The write-report skill will:
1. Pull structured data from `xreview report --session <session-id>`
2. Combine it with your conversation context (verification results, user decisions, fix details)
3. Generate a human-readable markdown report
4. Save it and present a summary to the user
5. Ask about session cleanup
```

- [ ] **Step 2: Verify the change**

Read `.claude/skills/xreview/SKILL.md` and confirm Step 5 now references write-report skill instead of directly running xreview commands.

---

### Task 3: Strengthen `--context` guidance in Step 1

**Files:**
- Modify: `.claude/skills/xreview/SKILL.md:57-83`

- [ ] **Step 1: Replace the `--context` assembly section**

In `.claude/skills/xreview/SKILL.md`, replace the current "Assembling `--context`" section (lines 57-83) with:

```markdown
### Assembling `--context`

The context string is critical — it tells Codex **what to focus on** and provides
background for the final report. Include as much relevant context as you have.

For **git-uncommitted** (change-focused):
```
--context "【背景】why this change is being made — the motivation or problem being solved
【變更類型】feature | refactor | bugfix
【描述】what was changed — specific functions, modules, or behaviors modified
【進度】current status — e.g. 'implementation complete, pre-commit review' or 'WIP, reviewing direction'
【預期行為】what this code should achieve (for refactor: 'behavior should be identical to before')
【未完成】anything not yet done or known limitations, if applicable"
```

For **files** (flow/feature review):
```
--context "【背景】why this review is needed — e.g. 'new feature ready for review', 'investigating production bug'
【Review 焦點】what to focus on — e.g. 'Review the CMS push event flow:
enqueue → EventQueue.push() → purge logic → SendQueue routing.
Focus on concurrency safety and lock correctness across these files.'
【進度】current status of the work
【預期行為】expected behavior — e.g. 'cache and ordered paths are fully independent, no cross-locking'"
```

For **files** (single file quality):
```
--context "【背景】why reviewing this file — e.g. 'recently refactored, want quality check'
【Review 焦點】General quality review of event_queue.cpp.
Look for bugs, race conditions, error handling issues.
【進度】current status"
```

The better the context, the better Codex's review AND the better the final report.
Be specific about the flow direction, expected behavior, and areas of concern.
Include background motivation — this gets stored in the session and used when
generating the review report.
```

- [ ] **Step 2: Verify the change**

Read `.claude/skills/xreview/SKILL.md` lines 57-90 and confirm the new context guidance includes 【背景】and 【進度】fields.

---

### Task 4: Commit

- [ ] **Step 1: Commit all changes**

```bash
git add .claude/skills/xreview/write-report.md .claude/skills/xreview/SKILL.md
git commit -m "feat: add write-report skill and improve review context guidance"
```
