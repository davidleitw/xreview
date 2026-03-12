# xreview Workflow Improvements — Feedback & Design Notes

Based on two rounds of end-to-end testing on a Go HTTP API project (6 files, ~400 lines).

## Test Results Summary

- **Round 1 test**: 15 findings across 5 rounds, all fixed. 10 high, 5 medium.
- **Round 2 test**: 19 findings across 6 rounds, all fixed. 9 high, 9 medium, 1 low.
- Both tests demonstrated the value of multi-round review but exposed efficiency issues.

---

## Problem 1: Drip-Fed Findings ("Toothpaste Problem")

**Observation**: Codex reveals new findings each round after fixes are applied. A review that should complete in 2 rounds (find → fix → verify) instead takes 5-6 rounds because each verification round surfaces additional issues.

**Examples**:
- Round 1 finds 8 issues. After fixing, Round 2 finds 3 more that were cascade effects of the original fixes (e.g., fixing an export endpoint exposed a format handling gap).
- An ownership check finding only appeared after a new route was added to fix a routing issue — this was a cascade of an earlier authorization finding.

**Root cause**: Two factors:
1. The first-round prompt does not explicitly instruct Codex to perform cascade analysis or check for pattern-wide issues.
2. The verification prompt allows Codex to report entirely new findings, not just verify fixes and check for regressions.

**Proposed fix (prompt layer)**:
- **First-round prompt**: Add explicit instruction: "For each finding, check whether the same pattern exists in other functions/files. Report all instances, not just the first one you see."
- **Verify prompt**: Restrict scope: "Confirm whether the listed fixes are correct and whether they introduced regressions. Do not report unrelated new findings."

---

## Problem 2: AskUserQuestion Too Ceremonial

**Observation**: Every round forces a full ceremony: verify each finding → present fix plan → AskUserQuestion A/B/C → wait for user → execute. For obvious one-line fixes (e.g., `defer rows.Close()`, parameterized query), this overhead is unnecessary.

**Examples**:
- A `defer rows.Close()` finding still required: read code → quote lines → classify → present fix plan with options → ask user → user picks A → apply.
- The user picked "A. Execute all recommended fixes" every single time across both tests.

**Proposed fix (skill + agent-instructions layer)**:
- Differentiate findings by decision complexity:
  - **Needs user decision**: Multiple fix approaches with trade-offs, or user might reasonably choose "don't fix" → full ceremony with AskUserQuestion
  - **Obvious fix**: Only one correct approach, no real alternative → include in fix plan but mark as "auto-fix", apply without waiting for per-finding approval
- Still present the full fix plan for review, but the AskUserQuestion options change:
  - A. Execute all (auto-fixes applied, decision-required fixes use recommended approach)
  - B. Let me review each decision-required finding individually
  - C. I want to adjust specific findings

---

## Problem 3: Finding Quality Degrades in Later Rounds

**Observation**: Round 1 findings are high-value real bugs (SQL injection, command injection, race conditions). Later rounds increasingly surface code quality nits or issues the developer already knew about (marked with `// BUG:` or `// TODO:` comments).

**Root cause**: After high-severity issues are fixed, Codex fills remaining capacity with lower-value observations. The prompt doesn't guide Codex to distinguish between "bugs that cause incorrect behavior" and "style/quality suggestions."

**Proposed fix (prompt layer)**:
- Add to first-round prompt: "Focus on bugs that cause incorrect behavior, security vulnerabilities, and data integrity issues. Do not report style issues, naming conventions, or issues that are already marked with TODO/BUG/FIXME comments."
- Consider a severity threshold: only report findings at medium severity or above by default.

---

## Problem 4: Verification Phase Redundant I/O

**Observation**: During Phase 1 (verify findings), Claude Code reads each file for every finding. If 7 findings span 4 files, the same files are read multiple times. Claude Code already read these files during the review — re-reading unchanged files is wasted I/O.

**Proposed fix (skill layer)**:
- Update agent-instructions or SKILL.md to allow: "If you have already read a file in this conversation and it has not been modified since, you may reference your prior reading instead of re-reading it."
- Group verification by file: read each file once, verify all findings in that file together.

---

## Problem 5: No Checkpoint Before Bulk Fixes

**Observation**: When user selects "Fix all," Claude Code applies fixes sequentially. If an error occurs mid-way (e.g., at finding 4 of 7), findings 1-3 are already modified. There is no rollback mechanism.

**Proposed fix (skill layer)**:
- Before executing fixes, instruct Claude Code to suggest a checkpoint: "Consider running `git stash` or creating a commit before applying fixes, so changes can be rolled back if needed."
- This is a suggestion in the skill, not enforced — keeps it lightweight.

---

## Problem 6: Missing "Dropped Findings" in Report

**Observation**: The write-report skill template has sections for Fixed, Dismissed, and Open findings. But there is no section for findings that were classified as SUSPECT during verification, challenged via Codex discussion, and dropped. This audit trail matters for post-review accountability.

**Proposed fix (skill layer)**:
- Add a "Dropped (False Positive)" section to the write-report template.
- Format: finding ID, original claim, Claude Code's reasoning, Codex's response, final decision.

---

## Problem 7: No Severity Adjustment Mechanism

**Observation**: Codex assigns severity unilaterally. After verification, Claude Code may determine that a "high" finding only triggers under very specific conditions and should be "medium." There is no formal mechanism to record this adjustment.

**Proposed fix (TBD)**:
- Could be a skill-layer convention (Claude Code notes adjusted severity in the fix plan).
- Could be a Go code change (add `adjusted_severity` field to findings).
- Low priority — the current workflow functions without this. Revisit if it becomes a pain point.

---

## Implementation Priority

| Priority | Problem | Layer | Effort |
|----------|---------|-------|--------|
| 1 | Drip-fed findings | Prompt templates | Small — wording changes |
| 2 | AskUserQuestion ceremony | Skill + agent-instructions | Medium — logic changes |
| 3 | Finding quality degradation | Prompt templates | Small — wording changes |
| 4 | Verification redundant I/O | Skill / agent-instructions | Small — wording changes |
| 5 | No checkpoint before fixes | Skill | Small — add suggestion |
| 6 | Missing dropped findings in report | write-report skill | Small — template change |
| 7 | Severity adjustment | TBD | Low priority |
