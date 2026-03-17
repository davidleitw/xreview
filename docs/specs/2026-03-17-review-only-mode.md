# Review-Only Mode Design

Date: 2026-03-17
Status: Draft

## Summary

Change `/review` default behavior from "review → fix plan → AskUserQuestion → fix → verify loop" to "review → present all findings → wait for user". The user reads findings, discusses freely, then selectively triggers fixes in conversation. Once fixes are triggered, the existing three-party consensus loop (Codex review → Claude Code verify → user decides) takes over unchanged.

## Motivation

The current workflow forces users into a rigid fix-or-skip decision immediately after review. This doesn't match natural development flow:

1. Developers want to **see the full picture first**, then decide what to act on.
2. The per-finding AskUserQuestion ceremony is heavy — obvious fixes don't need approval, and complex findings need discussion before a yes/no decision.
3. Users can't discuss findings with Claude Code (or ask Codex follow-up questions) before committing to a fix plan.

## Behavior Change

| Aspect | Before | After |
|--------|--------|-------|
| `/review` default | Verify → fix plan → AskUserQuestion → fix → verify loop | Verify → **present all findings → wait** |
| Enter fix phase | Automatic (part of the flow) | User-triggered in conversation ("修 F-001 和 F-003") |
| Discussion phase | None | Free conversation: ask questions, challenge findings, ask Codex for more |
| Fix scope | All confirmed findings at once | Only user-selected findings |
| AskUserQuestion | Mandatory gate before fixing | Removed from default flow |
| `--auto-fix` | N/A | Future work (for vibe coding workflows) |

## Detailed Workflow (Claude Code perspective)

### Step 0–2: Unchanged

Preflight, target selection, context assembly, and `xreview review` execution remain identical.

### Step 2.5: Verify + Present (changed)

**Phase 1: Verify Findings** — unchanged.

- Group by file, read each file once, classify CONFIRMED / SUSPECT.
- Challenge SUSPECT findings via `xreview review --session <id> --message "..."`.
- Drop confirmed false positives.

**Phase 2: Present All Confirmed Findings** — new behavior.

Present ALL confirmed findings to the user in this format:

```
### F-001 [HIGH/security] SQL injection in query builder
📍 db/query.go:42
> rows, err := db.Query("SELECT * FROM users WHERE id = " + userInput)
**Trigger**: user input passed directly to SQL string concatenation
**Impact**: attacker can read/modify any table
**Cascade**: affects all endpoints using BuildQuery()
**Suggested fixes**:
  A. (recommended) Use parameterized query — effort: minimal
  B. Add input sanitization — effort: moderate

### F-002 [HIGH/logic] nil pointer dereference
📍 handler/user.go:87
...
```

After ALL findings, output:

```
以上是本次 review 發現的 N 個問題。你可以：
- 討論任何 finding 的細節
- 告訴我要修哪些（例如「修 F-001 和 F-003」）
- 如果有其他想讓 Codex 檢查的方向，也可以告訴我
```

Then **stop and wait**. Do NOT auto-proceed. Do NOT use AskUserQuestion.

### Step 3: Discussion (new)

Free-form conversation. Claude Code handles user messages based on intent:

| User intent | Claude Code action |
|---|---|
| Ask about a finding ("F-001 的影響範圍?") | Read code, explain context |
| Challenge a finding ("F-002 上層已經檢查過了") | Re-verify, update classification |
| Ask Codex for more ("還有沒有其他 SQL injection?") | `xreview review --session <id> --message "..."`, verify new findings, present them |
| Request fixes ("修 F-001 和 F-003") | Enter Step 4 |

**Codex communication during discussion**: When the user's question requires Codex's judgment (scope beyond what Claude Code can answer from code alone), Claude Code forwards the question via `--session --message`. This uses the existing resume mechanism — no new CLI features needed. Any new findings from Codex go through Phase 1 verification before being presented.

### Step 4: Fix + Verify (existing three-party loop)

Once the user specifies which findings to fix:

1. Claude Code applies fixes for selected findings.
2. Run `xreview review --session <id> --message "Fixed F-001, F-003. Dismissed F-002 (reason). Verify fixed findings and check for new issues introduced by fixes."`.
3. If Codex finds new/reopened issues → verify and present them (same format as Step 2.5 Phase 2), user decides again.
4. Repeat until all selected findings are resolved or user says stop.
5. Maximum 5 rounds.

### Step 5: Finalize — unchanged

Invoke `write-report` skill, then `xreview clean`.

## Implementation Plan

### 1. `internal/formatter/xml.go` — rewrite `buildAgentInstructions()`

**Phase 1 (verify)**: Keep as-is.

**Phase 2**: Replace "Fix Plan + AskUserQuestion" block (current lines 116–161) with:

```
== PHASE 2: PRESENT ALL CONFIRMED FINDINGS ==

Present ALL confirmed findings to the user. For EACH:

1. Header: ### F-XXX [SEVERITY/category] title
2. Location: 📍 file:line
3. Code: > quote the code you read in Phase 1
4. Trigger: the trigger condition (from your verification, not copied from Codex)
5. Impact: what happens if exploited/triggered
6. Cascade: list cascade impacts
7. Suggested fixes: ALL alternatives with effort levels, mark recommended

After ALL findings, tell the user:
"以上是本次 review 發現的 N 個問題。你可以：
- 討論任何 finding 的細節
- 告訴我要修哪些（例如「修 F-001 和 F-003」）
- 如果有其他想讓 Codex 檢查的方向，也可以告訴我"

STOP here. Do NOT proceed to fix anything. Wait for user response.
```

**New Phase 3**: Add discussion + fix guidance:

```
== PHASE 3: DISCUSSION & FIX GUIDANCE ==

During discussion:
- User asks about a finding → read code and explain
- User's question needs Codex judgment → run: xreview review --session <SESSION_ID> --message "<question>"
  Then verify any new findings (Phase 1 process) and present them (Phase 2 format).
- User requests fixes (e.g. "修 F-001 和 F-003") →
  1. Apply fixes for specified findings only
  2. Run: xreview review --session <SESSION_ID> --message "Fixed [IDs]. Dismissed [IDs] (reasons). Verify fixes and check for new issues."
  3. If new/reopened findings → verify and present, user decides again
  4. Max 5 rounds
- When all work done → invoke write-report skill with session ID (write-report handles cleanup automatically)
```

### 2. `skills/review/SKILL.md` — update workflow description

Changes:
- **Step 2.5**: Remove "Fix Plan Gate" framing. Rewrite as "Verify + Present".
  - Remove Phase 2 "Build the Fix Plan" and "Get User Approval" sections.
  - Replace with "Present All Confirmed Findings" section matching the agent-instructions format.
  - Remove the instruction "you MUST end the fix plan with AskUserQuestion".
- **Step 3**: Rewrite as "Discussion" — describe how Claude Code handles different user intents (explain, challenge, ask Codex, request fixes).
- **Step 4**: Rewrite as "Fix + Verify" — triggered by user, applies only selected findings, then runs existing verify loop.
- **CRITICAL block** (lines 102–108): Update to remove AskUserQuestion mandate. Keep the verification mandate.
- **Important notes** (line 233): Update three-party review description — remove "present ALL findings as a Fix Plan and get user approval via AskUserQuestion BEFORE making any code changes", replace with "present ALL findings and wait for user to decide which to fix".

### 3. `internal/formatter/xml_test.go`

Update test cases for `buildAgentInstructions()` to match new Phase 2/3 content.

### Files NOT changed

- CLI commands and flags — no new flags needed
- `internal/session/` — session state management unchanged
- `internal/reviewer/` — Codex integration unchanged
- `skills/write-report/` — report skill unchanged
- Finding data structures — already have all needed fields (trigger, cascade_impact, fix_alternatives)

## Design Decisions

### Why no `--auto-fix` flag on Day 1

- Only one user currently. No backward compatibility concern.
- Adding `--auto-fix` means branching logic in both agent-instructions and skill — complexity for zero current demand.
- Recorded in README Future Work for vibe coding workflows.

### Why no `--review-only` flag

Review-only IS the default. There's nothing to flag.

### Why keep verification in review-only mode

Presenting unverified Codex output would make Claude Code a blind proxy. The value of xreview is the three-party model — Codex identifies, Claude Code verifies, user decides. Skipping verification defeats this.

### Why natural language triggers (not explicit commands) for fix phase

The skill provides instructions; Claude Code has the language understanding to recognize "修 F-001" as a fix request. The session ID and finding IDs are already in conversation context. Forcing `/review --fix --session <id>` adds friction for zero safety benefit.

The agent-instructions serve as fallback guidance — if context is compacted, Claude Code can still reference the session ID and fix flow described in the instructions it received.

### Why Codex communication during discussion

Users should be able to ask "are there other X risks?" without starting a new review session. The `--session --message` mechanism already supports this. Claude Code judges whether a question needs Codex (scope/expertise question) or can be answered by reading code (explanation question). This makes the discussion phase a true three-party conversation.
