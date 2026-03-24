# A1 + A2: Schema Enhancement & Skill UX Redesign

Date: 2026-03-19
Status: Design
Parent: `docs/specs/2026-03-18-evolution-discussion.md`

---

## Goal

Improve the review experience by:
1. Adding `confidence` and `fix_strategy` fields to the finding schema (A2)
2. Redesigning how the skill presents findings and handles fixes (A1)

These two changes are tightly coupled — the skill UX (A1) depends on the
schema fields (A2) to classify findings reliably.

---

## A2: Schema Enhancement

### New Fields

Add two fields to every finding:

| Field | Type | Values | Purpose |
|---|---|---|---|
| `confidence` | integer | 0-100 | How certain Codex is that this is a real issue |
| `fix_strategy` | string (enum) | `"auto"` \| `"ask"` | Whether this can be fixed without discussion |

Both fields are **required** in `review.json` (Codex must provide them).

### fix_strategy Classification Rules

Injected into the Codex prompt (FirstRoundTemplate) so Codex knows how to classify:

**"auto"** — a senior engineer would apply this without discussion:
- Dead code / unused variables
- Missing error check on a function that returns error
- N+1 query missing preload/join
- Obvious bug with single clear fix (nil dereference, off-by-one)
- Stale comment contradicting current code

**"ask"** — reasonable engineers could disagree:
- Security fixes with multiple mitigation strategies
- Design or naming decisions
- Behavior changes visible to users or callers
- Fix exceeds ~20 lines of change
- Race condition (multiple valid synchronization approaches)
- Any finding where confidence < 60

**Negative examples** (should NOT be "auto" even if they seem mechanical):
- Adding mutex to fix race condition → "ask" (lock granularity is a design decision)
- Changing public API signature → "ask" (breaking change affects callers)
- Removing a function parameter → "ask" (callers affected)

### Files Changed

#### 1. `internal/schema/review.json`

Add to the finding object properties:

```json
"confidence": {
  "type": "integer",
  "minimum": 0,
  "maximum": 100,
  "description": "How certain you are this is a real issue (0=guess, 100=certain)"
},
"fix_strategy": {
  "type": "string",
  "enum": ["auto", "ask"],
  "description": "auto: senior engineer would fix without discussion. ask: needs human decision."
}
```

Add `"confidence"` and `"fix_strategy"` to the `required` array.

#### 2. `internal/prompt/templates.go`

**FirstRoundTemplate:** Append to the existing field requirements section:

```
- confidence: 0-100. How certain you are this is a real issue.
  100 = you can see the exact bug. 50 = it looks suspicious but you're not sure.
  0 = pure speculation. Be honest — overconfidence wastes the verifier's time.
- fix_strategy: "auto" or "ask".
  "auto" = a senior engineer would apply this fix without discussion:
    dead code, missing error check, obvious single-fix bug, stale comment.
  "ask" = reasonable engineers could disagree on the approach:
    security trade-offs, design decisions, behavior changes, multi-approach fixes,
    anything where confidence < 60.
  When in doubt, use "ask".
```

**ResumeTemplate:** Update the previous findings format to include confidence
and fix_strategy so Codex can see and update its previous assessments:

```
[ID] (severity/category, confidence:N, strategy:auto|ask) file:line — description [status]
```

#### 3. `internal/session/types.go`

Add to the `Finding` struct (or equivalent):

```go
Confidence  int    `json:"confidence"`
FixStrategy string `json:"fix_strategy"`
```

Add to `CodexFinding` struct (the raw Codex output struct) similarly.

**Session versioning:** Add a `Version` field to the session struct:

```go
type Session struct {
    Version int `json:"version"`
    // ... existing fields
}
```

New sessions: `Version: 2`. On `Load()`: if version != 2 (or missing),
return an error. The caller creates a new session instead of resuming.
No migration, no silent fallback.

#### 4. `internal/formatter/xml.go`

**Two changes in this file:**

**(a) XML output:** Add `confidence` and `fix-strategy` as **attributes** on the
`<finding>` element. This is consistent with how other short metadata (`severity`,
`category`, `status`) are already attributes, while longer content (`trigger`,
`cascade-impact`) are child elements.

```xml
<finding id="F-001" severity="high" confidence="90" fix-strategy="auto">
  ...existing child elements unchanged...
</finding>
```

**(b) `buildAgentInstructions()` rewrite:** This function (lines 80-166) currently
embeds the full review workflow (Phase 1: verify, Phase 2: present, Phase 3: discuss/fix)
in Go code. **This must be rewritten to match the A1 workflow.**

Today there are two sources of truth for the workflow:
- `buildAgentInstructions()` in xml.go — emitted as `<agent-instructions>` after XML
- SKILL.md — the skill definition Claude Code loads

These already conflict (e.g., SKILL.md uses AskUserQuestion for approval;
agent instructions say "STOP here. Wait for user response" without AskUserQuestion).

**Decision: SKILL.md is the authoritative source.** `buildAgentInstructions()` should
be simplified to only emit:
- Session ID reference (for resume calls)
- Finding count summary
- A pointer: "Follow the workflow defined in your skill instructions."

The detailed Phase 1/2/3 workflow moves entirely to SKILL.md, eliminating the
dual-source-of-truth problem. This means `buildAgentInstructions()` shrinks from
~85 lines to ~10 lines.

**Rationale:** The skill is easier to iterate (no recompile), and having workflow
in two places guarantees they will drift. The agent instructions should only carry
information that the skill doesn't have (session ID, finding counts).

#### 5. `internal/prompt/builder.go`

**`FormatFindingsForPrompt()`**: Update to include confidence and fix_strategy
in the existing multi-line format. Keep all existing fields (Suggestion, Verification,
Trigger, Cascade, Fix alternatives) — they give Codex full context for re-evaluation.
Just add the new fields:

```
[ID] (severity/category, confidence:N, strategy:auto|ask) file:line — description [status]
  Suggestion: ...
  Verification: ...
  Trigger: ...
  Cascade: ...
  Fix A: ... [effort] (recommended)
```

#### 6. `internal/session/types.go` — `CodexFinding` struct

Add `Confidence` and `FixStrategy` to `CodexFinding` **without** `omitempty`.
These fields are `required` in the Codex output schema, so they must always
be present. Using `omitempty` would silently swallow a Codex compliance failure.

```go
Confidence  int    `json:"confidence"`
FixStrategy string `json:"fix_strategy"`
```

#### 7. Skill reference.md

Update the XML schema documentation to include the new attributes.

---

## A1: Skill UX Redesign

### Overview

Redesign the review skill's workflow to:
1. Present findings with full context (call path, consequences, impact)
2. Separate auto and ask findings in presentation
3. Auto findings are fixed directly after user sees the full picture
4. Detailed repair report after fixes are applied
5. Batch resume verification (once per batch, not per-finding)

### Revised Workflow

#### Step 2.5 Phase 1: Verify Each Finding (unchanged logic, structured output)

Claude Code reads code and verifies every finding. For each finding, Claude Code
internally records:
- Classification: CONFIRMED / FALSE POSITIVE / OVERRIDE
- Evidence: which file:line was read to verify
- If OVERRIDE: changed fix_strategy (e.g., Codex said "auto" but Claude Code
  thinks it needs discussion → override to "ask")

SUSPECT findings are challenged via `xreview review --session <id> --message "..."`.
This phase is not shown to the user — it is Claude Code's internal work.

#### Step 2.5 Phase 2: Present Findings with Full Context

After verification, Claude Code presents ALL confirmed findings at once.
If verdict is APPROVED (zero findings), skip directly to Step 5.

**Two presentation tiers based on fix_strategy:**

**Auto findings → table format (concise).** These are verified mechanical fixes.
The user needs to know what and where, not a full analysis.

**Ask findings → full analysis format.** These require user decision-making.
Each finding MUST include all 6 sections below.

**Ask finding required sections:**

1. **Problem**: one-sentence description
2. **Root cause**: why this issue exists — the underlying design or code decision
   that created the vulnerability. Not just "what's wrong" but "why it's wrong."
3. **Trigger path**: the concrete execution path that manifests the issue.
   Format: `function()` :line → what happens → next step → consequence.
4. **Consequence**: what happens if triggered in production.
   Be concrete (data loss, crash, security breach), not abstract.
5. **Likelihood assessment**: Claude Code's honest evaluation — is this a
   realistic production scenario, or a theoretical edge case? Consider:
   - Can we control the input to make this impossible?
   - Does the current architecture make this path reachable?
   - Under what conditions does this actually trigger?
   If Claude Code concludes the scenario is unlikely, say so and explain why.
   This is not a reason to drop the finding, but the user should know.
6. **Options**: fix approaches with trade-offs + "Don't fix" with stated risk.
7. **Impact scope**: what other code is affected by the fix. If none, say "none."

**Presentation format:**

```markdown
### Review 結果：N issues confirmed (M excluded as false positive)

#### 可直接修復（auto, X items）

| # | Location | Problem | Fix |
|---|----------|---------|-----|
| F-001 | handler.go:88 | SQL string interpolation | → parameterized query `$1` |
| F-003 | handler.go:15 | unused variable `tempResult` | → delete |
| F-005 | repo.go:30 | stale comment says "returns nil" | → update comment |

#### 需要你決定（ask, Y items）

---

##### F-002 [high] handler.go:42 — Race condition in cache update

**Problem:** Multiple goroutines read-modify-write OrderCache without synchronization.
**Root cause:** OrderCache is a plain map accessed from HTTP handlers. Each request
runs in its own goroutine, but the cache has no concurrency protection.
**Trigger path:** `HandleOrder()` :42 read cache → :45 append → :47 write back.
Two concurrent requests for same customer: goroutine A reads [1,2], B reads [1,2],
A writes [1,2,3], B overwrites to [1,2,4], order 3 lost.
**Consequence:** Order data loss under concurrent requests.
**Likelihood:** Realistic in production. Any two requests for the same customer
arriving within milliseconds of each other can trigger this. With a load balancer
distributing traffic, this will happen under moderate load. Not a theoretical edge case.
**Options:**
  A) sync.RWMutex — read lock / write lock around cache access (~5 lines)
  B) sync.Map — atomic LoadOrStore, better for read-heavy workload
  C) Don't fix — risk: order loss in production under concurrency
**Impact:** adds lock/unlock calls in HandleOrder(). Callers unaffected.

---

##### F-004 [medium] auth.go:55 — Timing side-channel in token comparison

**Problem:** Token comparison uses `==` instead of constant-time compare.
**Root cause:** Standard string comparison short-circuits on first mismatch,
leaking token length/prefix information via response timing.
**Trigger path:** `ValidateToken()` :55 → `if token == storedToken` → early return
on mismatch. Attacker measures response time for different token prefixes.
**Consequence:** Attacker can brute-force tokens character by character.
**Likelihood:** Low in practice. Requires network timing precision of ~nanoseconds.
Over the internet, network jitter masks the timing difference. Realistic only if
attacker has local network access or the service is latency-sensitive.
However, the fix is trivial and this is a well-known best practice.
**Options:**
  A) `subtle.ConstantTimeCompare()` — 1 line change, zero risk
  B) Don't fix — risk: theoretical timing attack, low practical likelihood
**Impact:** auth.go:55 only. No caller changes.

---
```

**Critical constraints:**
- Claude Code must NOT skip any required section for ask findings.
- For the **likelihood assessment**, Claude Code must think critically:
  is this a real production risk or a theoretical concern? If the scenario
  requires conditions that are unlikely or controllable, say so honestly.
  The user deserves to know "this is real and urgent" vs "this is best practice
  but unlikely to be exploited in your context."
- If Claude Code cannot determine the root cause or trigger path, write
  "Root cause unclear — Codex reports: [original description]" so the user
  knows this was not independently verified.

#### Step 2.5 Phase 3: User Decision

After presenting all findings, use AskUserQuestion. **Options adapt to the
auto/ask mix** — don't show irrelevant choices:

**When both auto and ask findings exist:**
```
N findings confirmed (X auto, Y ask).

Auto items (F-001, F-003, F-005): verified, ready to fix.
Ask items (F-002, F-004): need your choice.

A. Fix all (auto directly + ask use recommended)
B. Fix auto only, discuss ask items after
C. Review only — don't fix anything yet
Or type adjustments, e.g. "F-002 B", "F-004 skip"
```

**When only auto findings:**
```
N findings confirmed, all auto-fixable.

A. Fix all (will provide detailed report after)
B. Review only — don't fix anything yet
Or type adjustments, e.g. "F-003 skip"
```

**When only ask findings:**
```
N findings confirmed, all need your decision.

A. Fix all using recommended options
B. Review only — don't fix anything yet
Or type your choices, e.g. "F-002 B", "F-004 skip"
```

**Key difference from current workflow:** The user has already seen the full
context for every finding. Auto items were shown in a concise table; ask items
had full analysis with root cause, likelihood, and options. The decision is
informed — auto items don't need per-item confirmation because the user has
seen exactly what will change and why.

#### Step 3: Execute Fixes (modified)

Apply fixes per the user's decision:
- All approved auto fixes in one batch
- All approved ask fixes in one batch
- No per-finding resume between fixes

#### Step 3.5: Repair Report (new)

After all fixes are applied, present a concise report. Don't repeat problem
descriptions — the user already saw those in Phase 2. Focus on what changed.

```markdown
### Repair Report

| Finding | Location | Fix Applied |
|---------|----------|-------------|
| F-001 | handler.go:88 | → parameterized query `$1` |
| F-003 | handler.go:15 | → deleted unused assignment |
| F-005 | repo.go:30 | → updated comment |
| F-002 | handler.go:42 | → sync.RWMutex around cache access |

`git diff` to see full changes.
```

The report serves as:
- Confirmation of what was done (user can verify against git diff)
- Input for the resume verification message
- Documentation trail for the review session

#### Step 4: Resume Verification (modified)

**One resume call after all fixes**, not per-finding:

```
xreview review --session <id> --message "Fixed F-001 (parameterized query),
F-003 (deleted unused var), F-002 (sync.RWMutex). Please verify all fixes
and check for regressions."
```

The message includes the repair report content so Codex knows exactly what changed.

Parse result:
- All resolved → Step 5 (finalize)
- New issues or regressions → present with same format (call path, consequence, etc.),
  get user decision, fix, resume again
- Max 5 rounds → inform user, proceed to Step 5

---

## Backward Compatibility

### Session Format

Sessions created before this change have no `version` field, no `confidence`,
no `fix_strategy` in their findings.

**Policy:** No backward compatibility. If session version doesn't match,
return error and start a new session. Users see:
"This session was created with an older version of xreview. Starting a new review."

### Skill + Agent Instructions

The workflow moves from dual-source (SKILL.md + `buildAgentInstructions()` in Go)
to single-source (SKILL.md only). `buildAgentInstructions()` is simplified to a
minimal pointer. This is a breaking change in how xreview communicates workflow
to Claude Code, but since both sources already conflicted, this resolves rather
than creates an inconsistency.

**Note:** The parent discussion document (`2026-03-18-evolution-discussion.md`)
mentioned `omitempty` for backward compatibility. This spec supersedes that
approach — we use session versioning with hard break instead. The parent doc
should be updated to note this revision.

### XML Output

New XML attributes (`confidence`, `fix-strategy`) are additive. Any consumer
that ignores unknown attributes is unaffected.

---

## What Does NOT Change

- Step 0 (Preflight): unchanged
- Step 1 (Determine targets + assemble context): unchanged
- Step 2 (Run review): unchanged — `xreview review` command is the same
- Step 5 (Finalize / write-report): unchanged
- CRITICAL_RULES in Codex prompt: unchanged
- Three-party model (Codex reviews, Claude Code verifies, user decides): unchanged
- SUSPECT finding challenge flow: unchanged (but the classification set is expanded
  to include OVERRIDE — this is additive, the existing CONFIRMED/SUSPECT flow is intact)
- Max 5 rounds limit: unchanged

---

## Step Mapping: Current → New

| Current SKILL.md | New SKILL.md | Change |
|---|---|---|
| Step 0: Preflight | Step 0: Preflight | Unchanged |
| Step 1: Determine targets + context | Step 1: Determine targets + context | Unchanged |
| Step 2: Run review | Step 2: Run review | Unchanged |
| Step 2.5 Phase 1: Verify each finding | Step 2.5 Phase 1: Verify each finding | Logic unchanged; adds OVERRIDE classification |
| Step 2.5 Phase 2: Build fix plan | Step 2.5 Phase 2: Present with full context | **Rewritten** — 5 required sections per finding (problem, call path, consequence, fix, impact) |
| Step 2.5: Get user approval (AskUserQuestion) | Step 2.5 Phase 3: User decision | **Simplified** — A/B/C + free-form input |
| Step 3: Execute fixes | Step 3: Execute fixes | **Modified** — batch all fixes, no per-finding resume |
| *(none)* | Step 3.5: Repair report | **New** — structured table of all changes |
| Step 4: Summary + Verify | Step 4: Resume verification | **Modified** — single resume after batch, not per-finding |
| Step 5: Finalize | Step 5: Finalize | Unchanged |

---

## Session Version Check: Boundary of Responsibility

The version check lives in `session/manager.go` `Load()`:

```go
func (m *manager) Load(sessionID string) (*Session, error) {
    // ... read and unmarshal JSON ...
    if sess.Version != CurrentVersion {
        return nil, fmt.Errorf("session %s uses schema version %d (current: %d)",
            sessionID, sess.Version, CurrentVersion)
    }
    return sess, nil
}
```

The caller (`internal/reviewer/single.go` `Resume()`) handles the error:
- If `Load()` returns version error → return a specific error type
- `cmd_review.go` catches this error → outputs XML with `<error>` tag:
  "This session was created with an older version of xreview. Starting a new review."
- The skill instructs Claude Code to re-run `xreview review` (new session) when
  it sees this error.

The user is never asked to decide — the skill handles it automatically.

---

## Design Principles Applied

1. **User sees the full picture before deciding.** Ask findings have root cause,
   trigger path, consequence, likelihood, and options. Auto findings have a
   concise table. No black-box "trust me, this is a bug."

2. **Honest likelihood assessment.** Claude Code must think critically about
   whether a finding is a real production risk or a theoretical edge case.
   Can the input be controlled? Is the trigger path actually reachable?
   The user deserves "this is urgent" vs "this is best practice but unlikely."

3. **Auto ≠ invisible.** Auto findings are shown in a table — visible but concise.
   The user sees what will change, batch-approves, and gets a repair report after.
   The difference from ask is presentation density, not visibility.

3. **Batch over drip-feed.** All findings at once. All fixes at once.
   One resume verification. Reduces Codex calls and user wait time.

4. **Claude Code verification is the quality gate.** If Claude Code confirms
   a finding and agrees with fix_strategy=auto, the classification is trusted.
   Claude Code can override Codex's fix_strategy if it disagrees.

5. **Repair report as documentation.** The user always knows exactly what
   changed, where, and why. No silent modifications.

---

## Risk Assessment

| Risk | Likelihood | Mitigation |
|---|---|---|
| Codex misclassifies security fix as "auto" | Low (prompt rules are explicit about security → ask) | Claude Code verification can override; user still sees full context before approving |
| Codex fails to produce confidence/fix_strategy | Very low (required + additionalProperties:false) | If it happens, review fails with clear error rather than silently degrading |
| Auto fix introduces regression | Low (auto fixes are mechanical by definition) | Step 4 resume verification catches regressions |
| User approves auto batch without reading | User's choice | Full context is presented — we provide information, user decides how much to read |
| Old sessions can't be resumed | Expected | Clear error message, start new session. Old sessions are rare (most reviews complete in one sitting) |
