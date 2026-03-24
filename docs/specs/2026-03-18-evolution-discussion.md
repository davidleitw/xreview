# xreview Evolution — Discussion Notes & Design Direction

Date: 2026-03-18
Status: Discussion (pre-design)

This document captures the discussion around xreview's next evolution.
It consolidates research, trade-off analysis, and design decisions made
during brainstorming sessions, serving as input for implementation specs.

---

## Starting Point

xreview v0.8 provides single-round or multi-round code review:

```
User → Skill → xreview review → 1 Codex exec → findings → Claude Code verifies → User
```

Current scope modes: `--files` and `--git-uncommitted`.
Context: a single `--context` string injected into the Codex prompt.
Schema: findings with severity, category, trigger, cascade_impact, fix_alternatives.

---

## Original Roadmap vs Revised Direction

The original roadmap (`2026-03-17-roadmap-next-generation-review.md`) proposed a
5-phase plan centered on context engineering and multi-angle review. After critical
analysis, several elements were revised or deferred.

### What Changed and Why

| Original Proposal | Revised Decision | Reasoning |
|---|---|---|
| Claude Code pre-gathers cross-file context (grep symbols, trace calls) into structured context-file | Context-file carries only low-cost existing info | No surveyed tool does this; Anthropic's data suggests more context = more false positives; high token cost for speculative gathering |
| Context-file has required fields (project_summary, files_under_review, review_hints) | No required fields, no schema enforcement | Most fields are derivable from code itself ("this is an HTTP handler" is obvious from the code); only write what Codex can't see on its own |
| Two-pass review (critical → informational) as standard strategy | Single Codex call as default; --focus for targeted review | Doubles cost; unproven benefit; --focus achieves similar effect on demand |
| Claude Code auto-selects strategy (quick/standard/deep) | User decides; no auto-escalation | Adds complexity to skill; cost should be predictable; most reviews need one pass |
| Checklist YAML system (.xreview/checklists/) per language | Deferred; hardcode in prompt first | Over-engineered for current stage; validate concept before building infrastructure |
| Multi-angle review in Phase 2 | Deferred to Phase 3 (observe-then-decide) | High complexity (session-group, merge, dedup); need evidence that single review + good context isn't enough |

### What Was Added

| New Item | Source | Reasoning |
|---|---|---|
| `fix_strategy` field (auto/ask) per finding | gstack borrowing | Enables batch presentation of mechanical fixes; better UX than treating all findings equally |
| `confidence` field per finding | Original roadmap (kept) | Low-confidence findings get extra verification scrutiny |
| Taste review as separate command | Discussion about naming/style review | Bug review and taste review have fundamentally different prompts, schemas, and checklists; mixing them dilutes both |
| Language-specific checklists embedded in binary | Discussion about C++ Core Guidelines | Taste review needs language-specific knowledge; Go embed is zero-cost distribution |

### Token Cost Analysis That Drove Decisions

| Change | Extra Token Cost | Benefit |
|---|---|---|
| Skill UX improvement | 0 | Better presentation |
| Schema confidence + fix_strategy | ~0 (few prompt lines) | Structured findings |
| --focus flag | ~0 (one prompt sentence) | Directed review |
| --git-diff | 0 (different file collection method) | Convenience |
| --context-file (lightweight) | Low (existing info only) | Extensibility |
| --context-file (cross-file gathering) | **High** (many grep/read) | Possibly more false positives |
| Two-pass review | **Codex cost ×2** | Unproven |
| Multi-angle review | **Codex cost ×3** | Unproven |

The highest-ROI changes are all zero or near-zero token cost.
The expensive changes solve problems that may not be frequent enough to justify the cost.

---

## Research: How Other Tools Handle Context

### Surveyed Tools

| Tool | Context Strategy |
|---|---|
| **Anthropic official /code-review plugin** | CLAUDE.md discovery + PR metadata. Bug agents see ONLY the diff — deliberately limited context to reduce false positives. Validation subagents re-verify each finding. |
| **awesome-skills/code-review-skill** | Phase 1 context gathering (PR description, CI status, business requirement). Progressive loading: language-specific guides load on-demand. |
| **levnikolaevich/claude-code-skills** | Goal Articulation Gate — reviewer must state REAL GOAL, DONE criteria, NOT THE GOAL before reviewing. |
| **gstack /review** | Two-pass (critical → informational). Per-finding AUTO-FIX vs ASK classification. Suppression rules. Checklist-driven. |
| **ChrisWiles** | PR diff + project-specific checklist file. |
| **Qodo/PR-Agent** | PR-scoped, PR history + codebase context. |

### Key Finding

**No tool does "pre-gather cross-file context for the reviewer."**

All context is lightweight and already-existing:
1. **Author intent** — PR description, commit message, linked issues
2. **Project standards** — CLAUDE.md, coding convention files
3. **Suppression rules** — what NOT to flag

Anthropic's official plugin deliberately restricts context to the diff only for
bug-finding agents, reasoning that more context increases false positives.

### Implications for xreview

The original roadmap proposed having Claude Code grep symbols, trace call chains,
and write structured context-files before review. This approach:
- Has no precedent in any surveyed tool
- Is expensive (many Claude Code tool calls to gather context)
- May increase false positives (per Anthropic's experience)
- Solves a problem (semantic gap issues) that may not be frequent enough to justify the cost

**Decision: context-file should carry low-cost already-existing information, not
expensive pre-gathered cross-file analysis.** Claude Code can do targeted cross-file
investigation during the verification phase, where it has a specific finding to
confirm — this is more efficient than speculative pre-gathering.

---

## Research: gstack Patterns Worth Borrowing

### 1. Per-Finding Fix Strategy (AUTO-FIX vs ASK)

gstack classifies each finding:
- **AUTO-FIX**: mechanical fixes a senior engineer wouldn't discuss (dead code, N+1, obvious bug)
- **ASK**: design decisions, behavior changes, security trade-offs

Rule of thumb: "If a senior engineer would apply the fix without discussion, it's AUTO-FIX."

**Adopted as:** `fix_strategy` field in xreview finding schema.

### 2. Suppression Rules

gstack maintains an explicit "DO NOT flag" list:
- Harmless redundancy aiding readability
- Issues already addressed in the current diff
- Assertions already covering behavior

**Deferred:** Will monitor false positive frequency first. If it becomes a measurable
problem, implement `.xreview/config.json` suppressions injected into Codex prompt.

### 3. Two-Pass Review (Critical → Informational)

gstack separates review into Pass 1 (security, data safety) and Pass 2 (naming, dead code).

**Reconsidered:** Two-pass doubles Codex cost. Most daily reviews don't need it.
The `--focus` flag achieves a similar effect when the user wants targeted review,
without forcing two passes every time.

---

## Design Decisions

### Context-File: Minimal, Not Maximal

Original proposal: Claude Code gathers cross-file structural context before every review.

Revised decision: `--context-file` is a dumb pipe (read file, inject into prompt).
Skill instructs Claude Code to only put low-cost information in it:
- Commit message or PR description (if available)
- Relevant project rules from CLAUDE.md
- User-stated background
- Suppression rules (future)

No symbol grep, no call chain tracing, no structured schema enforcement.
If Claude Code has nothing useful to add, skip context-file entirely.

### Focus vs Context: Separate Concerns

- `--context` / `--context-file` = what the reviewer should **know** (background)
- `--focus` = what the reviewer should **look for** (instruction)

These are injected into different sections of the Codex prompt.

### Single Codex Call as Default

No automatic multi-pass, no automatic strategy selection by Claude Code.
Default is always one Codex review call. More passes only when the user
explicitly requests deep/thorough review. This keeps cost predictable.

### Taste Review: Separate Command, Not a Mode

Naming/readability/idiomatic review is fundamentally different from bug review:
- Different prompt (role, instructions, what NOT to report)
- Different schema (no trigger, no cascade_impact)
- Different checklist (language-specific)

Implemented as `xreview review-taste`, not a flag on `xreview review`.
Language checklists embedded in the binary via `//go:embed`.
User can add project-specific guidelines via `--guideline <path>`.

### Proactive vs Reactive Context Strategy

Two approaches were considered for cross-file context:

**Proactive (rejected):** Claude Code gathers structural context before review.
- Grep all symbols, trace call chains, write structured JSON
- Problems: expensive, speculative, may increase false positives
- No surveyed tool does this

**Reactive (adopted):** Claude Code investigates cross-file context during verification.
- First round: Codex reviews with diff only (or with lightweight context-file)
- Verification: Claude Code reads code to confirm each finding
- If a finding needs cross-file info to verify → Claude Code greps/reads at that point
- If Claude Code suspects Codex missed something → second round with --focus

This is more efficient because investigation is targeted (specific finding to confirm)
rather than speculative (gather everything that might be useful).

### xreview CLI is Not User-Facing

All xreview commands are invoked by Claude Code via the skill, never by the user directly.
This affects design decisions:

- `--focus` is determined by Claude Code based on user intent, not typed by user
- `--context-file` is written by Claude Code, not by user
- `--language` for taste review is auto-detected by Claude Code
- Error messages from xreview are consumed by Claude Code and translated for user
- CLI flag ergonomics matter less than semantic clarity for an AI consumer

---

## Evolution Roadmap

### Phase 1: Improve Existing Review (Direction A)

Small, high-ROI changes to make the current review experience better.

#### A1. Skill UX Improvement

- **What:** Restructure how findings are presented to user
- **Where:** SKILL.md only (zero CLI changes)
- **How:** Group findings by fix_strategy — "auto" items presented as batch
  ("I can fix these directly, proceed?"), "ask" items listed individually
  with fix options. Simplified AskUserQuestion format. Batch fixes before
  resuming verification (don't resume per-finding).
- **Solves:** "Findings drip-fed across rounds", "AskUserQuestion too ceremonial"
- **Current Skill location:** `~/.claude/plugins/marketplaces/xreview-marketplace/skills/review/SKILL.md`
- **Concrete changes to Skill:**
  - Step 2.5 Phase 2 (Build Fix Plan): instead of listing all findings uniformly,
    split into two sections:
    ```
    ### Can fix directly (fix_strategy: auto)
    - F-001: Unused variable `tempResult` in handler.go:15 → delete
    - F-003: Missing error check on json.Marshal in handler.go:78 → add if err != nil

    ### Needs your decision (fix_strategy: ask)
    - F-002: Race condition in cache update (handler.go:42)
      Options: A) sync.RWMutex B) sync.Map C) Don't fix
    ```
  - AskUserQuestion simplified from current verbose format to:
    ```
    2 auto-fix, 1 needs decision. Enter=fix all | S=skip | or adjust (e.g. "F-002 skip")
    ```
  - Step 3 (Execute Fixes): apply all approved auto-fixes in one batch, then
    all approved ask-fixes. Resume verification once after all fixes, not per-finding.
  - Step 4 (Verify): single `xreview review --session <id> --message "..."` call
    covering all fixes in the batch.

#### A2. Schema: confidence + fix_strategy

- **What:** Add two fields to finding schema
- **Where:** review.json, templates.go, SKILL.md
- **How:**
  - `confidence` (0-100): Codex's certainty this is a real issue
  - `fix_strategy` ("auto" | "ask"): Codex's recommendation for handling
  - Prompt includes classification rules for Codex
  - Skill uses confidence for verification priority (< 60 gets extra scrutiny)
  - Skill uses fix_strategy for presentation grouping
- **Solves:** Gives A1's presentation logic a reliable signal from Codex
- **Backward compat:** Existing sessions lack these fields. Go structs use `omitempty`
  for new fields; old sessions default to confidence=0, fix_strategy="" (treated as "ask"
  by skill — safe default, never auto-fixes without explicit signal).
- **Files to change:** review.json, templates.go, session/types.go (Go structs),
  formatter/xml.go (XML output), SKILL.md

**fix_strategy classification rules (injected into Codex prompt):**

"auto" — a senior engineer would apply this without discussion:
- Dead code / unused variables
- Missing error check on a function that returns error
- N+1 query missing preload/join
- Obvious bug with single clear fix (nil dereference, off-by-one)
- Stale comment contradicting code

"ask" — reasonable engineers could disagree:
- Security fixes (multiple mitigation strategies)
- Design/naming decisions
- Behavior changes visible to users
- Fix exceeds ~20 lines
- Race condition (multiple valid synchronization approaches)
- Any finding with confidence < 60

Negative examples (should NOT be "auto" even if they seem mechanical):
- Adding mutex to fix race condition → "ask" (lock granularity is a design decision)
- Changing public API signature → "ask" (breaking change)
- Removing a function parameter → "ask" (callers affected)

#### A3. --focus Flag

- **What:** Separate "review direction" from "background context"
- **Where:** cmd_review.go, templates.go, SKILL.md
- **How:**
  - New flag `--focus <string>`, injected into prompt instruction section
  - Skill teaches Claude Code when to add focus:
    - User specifies direction → translate to --focus
    - Claude Code observes high-risk patterns → add --focus
    - No observation → don't add, let Codex review broadly
  - xreview is called via skill, not by users directly, so --focus is
    always determined by Claude Code based on user intent
- **Solves:** Review direction controllable without contaminating context
- **Enables:** Future multi-angle review (each angle = different --focus)

#### A4. --git-diff Scope Mode

- **What:** Review changes relative to a git ref
- **Where:** cmd_review.go, single.go, SKILL.md
- **How:**
  - New flag `--git-diff <ref>`, mutually exclusive with --files/--git-uncommitted
  - xreview runs `git diff <ref> --name-only` for file list
  - FetchMethod: primary approach is xreview passes diff content directly in prompt
    (avoids dependency on Codex sandbox having git history). Fallback: instruct Codex
    to run `git diff <ref>` if repo is available in sandbox (needs verification).
  - Supports: `HEAD~3`, `origin/main`, `abc123..def456`, single commit hash
  - Skill mapping: "review this PR" → `--git-diff origin/main`,
    "review last 3 commits" → `--git-diff HEAD~3`,
    "review this commit" → `--git-diff <hash>~1..<hash>`
- **Solves:** Most common use case (review branch changes) requires manual file listing today
- **Note:** This is the answer to "can I review a specific commit?" — currently not supported

#### A5. --context-file (Lightweight)

- **What:** File-based context injection, replacing string length limits
- **Where:** cmd_review.go, builder.go, SKILL.md
- **How:**
  - New flag `--context-file <path>`, reads file, injects into prompt context section
  - CLI does no validation, no schema enforcement — pure passthrough
  - Can coexist with --context (context-file content appears first)
  - Skill: only put low-cost already-existing info (commit msg, PR desc, project rules)
  - Explicitly: do NOT pre-gather cross-file context
- **Solves:** --context string length limit, future extensibility

**Execution order:**
```
A1 + A2 (together — presentation depends on fix_strategy)
  → A3 + A4 (together — both new flags)
  → A5 (independent — placed last because A3/A4 work fine without it;
         context-file adds richer context but is not required for focus or git-diff)
```

### Phase 2: Taste Review (Direction B)

Independent review dimension for naming, readability, and idiomatic style.

#### B1. xreview review-taste Command

- **What:** New CLI command for taste/style review
- **Where:** New cmd_review_taste.go, new prompt template, new schema, embedded checklists
- **How:**
  - `xreview review-taste --files <paths> --language <lang> [--guideline <path>]`
  - Language auto-detected from file extensions if not specified
  - Built-in checklists via `//go:embed` (Go, C++, TypeScript, Python, Rust)
  - Each checklist: 50-100 most common bad taste patterns for that language
  - Custom guidelines via `--guideline` appended after built-in checklist
  - Dedicated prompt template: reviewer role is "code taste", explicitly NOT bugs
  - Simplified finding schema: no trigger, no cascade_impact, no fix_alternatives
    (taste fixes are usually straightforward renames/restructures)
  - Finding categories: naming, readability, consistency, idiom

**Built-in checklist examples:**

C++ (from Core Guidelines):
- Naming conventions (PascalCase classes, no Hungarian notation)
- RAII naming should reflect ownership
- Const correctness
- Avoid implicit conversions
- Template parameter naming

Go (from Effective Go):
- Package-function name stuttering
- Receiver naming conventions
- Error variable naming
- Exported vs unexported naming intent
- Interface naming (-er suffix)

#### B2. Taste Review Skill

- **What:** Skill section or separate skill for taste review
- **Where:** xreview-marketplace skills
- **How:**
  - Triggered by: "check naming", "review code taste", "review style"
  - NOT triggered by default "review" (bug review only by default)
  - "review everything" / "complete review" → bug review + taste review
  - Claude Code detects language from file extensions
  - Checks for `.xreview/taste-guideline.md` (project custom guidelines)
  - Verification: Claude Code confirms naming is actually misleading in context,
    not just technically non-idiomatic

### Phase 3: Advanced Features (Direction C — Observe Then Decide)

Only pursue after Phase 1/2 are stable and usage patterns are clear.

#### C1. Multi-Angle Review (session-group + merge)

- **When:** If users frequently re-run reviews with different --focus values
- **Signal:** "I wish I could run security review and naming review in parallel"
- **Complexity:** High — session-group, merge command, dedup logic

#### C2. Suppression Rules / Review Memory

- **When:** If false positives are frequent and repetitive (same pattern across reviews)
- **Signal:** Users repeatedly dismissing the same finding type
- **Approach:** `.xreview/config.json` suppressions injected into Codex prompt

#### C3. Multi-Model Review

- **When:** Other models offer comparable sandbox + structured output capabilities
- **Signal:** Gemini or other models can read codebase files and return structured JSON
- **Approach:** Reviewer interface abstraction, `--reviewer <backend>` flag

#### C4. Design Plan Review

- **When:** User demand for reviewing design docs, not code
- **Complexity:** High — entirely different prompt, schema, finding structure

---

## Deep Dive: Context-File Design Evolution

This was the most extensively discussed topic. The thinking evolved through several stages.

### Stage 1: Structured Context with Required Fields

Original idea from roadmap: Claude Code writes a structured JSON with required fields
(project_summary, files_under_review, review_hints) and optional fields
(symbol_cross_references, key_data_structures, lifecycles).

**Problem identified:** Most "required" fields are low-value.
- `project_summary`: Codex sees `http.HandleFunc` + `sql.Query`, it knows it's an HTTP API + SQL.
  Telling it "this is an HTTP API" is redundant.
- `files_under_review`: Codex sees `func (r *Repository) GetOrder(...)`, it knows this is
  repository layer. Describing each file's role is usually obvious from the code.
- `review_hints` "at least one": Forces Claude Code to write something even when there's
  nothing non-obvious to say. Wastes tokens on filler.

### Stage 2: "Only Write What Codex Can't See"

Revised rule: only include observations that Codex cannot derive from the diff alone.

Good context: "handler.go:42 and repo.go:120 both use kMaxCacheSize, but one compares
per-customer list length, the other is a SQL LIMIT"

Bad context: "handler.go handles HTTP requests" (obvious from code)

**Problem identified:** This requires Claude Code to grep symbols and trace usage —
the expensive pre-gathering we wanted to avoid. And if Claude Code can identify the
cross-file inconsistency, it's already doing the review work that Codex should do.

### Stage 3: Low-Cost Existing Information Only (Final Decision)

After surveying other tools and finding that none do pre-gathering:

Context-file should only contain information that already exists and costs nothing to collect:
- Commit message / PR description
- Relevant CLAUDE.md project rules
- User-stated background and intent
- Suppression rules (future)

If none of these exist or are relevant, skip context-file entirely.

Cross-file investigation happens reactively during Claude Code's verification phase,
where it has a specific finding to confirm — targeted, not speculative.

### Key Insight from Anthropic's Plugin

Anthropic's official code-review plugin tells its bug-finding agents:
> "Do not flag issues that you cannot validate without looking at context outside of the git diff."

This is the opposite of "give more context." Their experience shows that restricting
context to the diff improves precision. The reviewer that sees less but stays within
its competence produces better results than one that sees more but hallucinates connections.

---

## Open Questions

### Resolved

1. **Context-file format?**
   → No format enforced by CLI — it reads the file and injects contents as-is (plain text,
   JSON, Markdown, anything works). Skill convention: Claude Code writes plain text or JSON
   at its discretion. No parsing, no validation on the CLI side.

2. **Context-file schema enforcement?**
   → No. CLI reads file, injects contents. No validation, no required fields.
   Quality control is in the skill (what Claude Code puts in), not the CLI.

3. **Who decides review strategy (quick/standard/deep)?**
   → Not automated. Default is always single Codex call. User explicitly requests
   deeper review. No Claude Code auto-escalation.

4. **Two-pass review as default?**
   → No. Doubles cost, unproven benefit. --focus achieves similar effect on demand.

5. **Where do taste checklists live?**
   → CLI binary (Go embed). Skill only decides when to call and what parameters.

### Still Open

1. **Taste review finding schema:** Mostly resolved. Drop trigger, cascade_impact,
   fix_alternatives (taste fixes are straightforward renames). Keep confidence and
   fix_strategy. Add `rationale` field (why this matters for maintainability).
   The `rationale` field is tentative — validate with real usage to confirm it
   adds value beyond `description`.

2. **--git-diff and Codex sandbox:** Primary approach: xreview runs `git diff` locally
   and passes diff content to Codex via prompt (no dependency on Codex sandbox git).
   Open question: should Codex also get full file contents (for context beyond the diff),
   or is diff-only sufficient? Current `--git-uncommitted` mode instructs Codex to run
   git commands itself — need to decide if `--git-diff` follows the same pattern or
   pre-computes everything locally.

3. **Language detection:** Auto-detect from file extensions is simple but imperfect
   (`.h` could be C or C++). Fallback to `--language` flag. Is this good enough?

4. **Taste checklist maintenance:** Who maintains the built-in checklists? How do
   they get updated? Ship with xreview releases? Accept community PRs?

5. **"Review everything" orchestration:** When user says "complete review", skill
   needs to run bug review AND taste review sequentially (or parallel?). How does
   the final report combine both? Separate reports or unified?

---

## Critical Analysis: Five Perspectives on "How to Improve xreview"

During brainstorming, we examined the problem from multiple angles to avoid
anchoring on context-file as the only solution.

### 1. User Experience Perspective

The biggest friction is not review quality — it's review flow.
- Findings drip-fed one at a time across rounds
- AskUserQuestion is too ceremonial for obvious fixes
- Each fix triggers a resume round (1-2 min wait)

**Implication:** Skill UX improvement (A1) may have higher impact than any CLI feature.

### 2. Review Quality Perspective

"Codex misses cross-file semantic issues" was the roadmap's starting premise.
But how frequent is this? If most reviews are single-file bug fixes, the
semantic gap problem is rare. Investing heavily in context for a rare problem
is poor ROI.

**Implication:** Fix the common case first (better UX, confidence scoring),
then observe whether semantic gap issues are actually frequent.

### 3. Token Economics Perspective

| Change | Cost | Benefit |
|---|---|---|
| Skill-only changes | 0 | High (UX) |
| Schema changes | ~0 | High (structure) |
| Lightweight context-file | Low | Medium (extensibility) |
| Cross-file gathering | High | Uncertain |
| Multi-pass | ×2-3 Codex cost | Uncertain |

Highest ROI changes are all zero-cost. Expensive changes solve uncertain problems.

### 4. Competitive Moat Perspective

xreview's moat is cross-model diversity + Claude Code verification + user-in-loop.
Not context depth. CodeRabbit/Greptile/Augment have infrastructure (vector DBs,
semantic indexes) that xreview can never match as a local-first tool.

**Implication:** Strengthen the moat (better verification, better UX, taste review
as new dimension) rather than chase context infrastructure.

### 5. Implementation Cost Perspective

xreview is maintained by one person. Every feature needs Go code, tests, maintenance.

| Feature | Go Code | Skill | Maintenance |
|---|---|---|---|
| Skill UX | None | ~50 lines | Low |
| Schema fields | ~10 lines | ~20 lines | Low |
| --focus | ~20 lines | ~15 lines | Low |
| --git-diff | ~40 lines | ~20 lines | Low |
| --context-file | ~25 lines | ~15 lines | Low |
| review-taste | ~200 lines | ~40 lines | Medium (checklists) |
| session-group + merge | ~500+ lines | ~50 lines | High |

Phase 1 (A1-A5) total: ~100 lines Go + ~120 lines skill. Very manageable.

---

## Appendix: Competitive Landscape Reference

| Tool | Architecture | Cross-File | Context Strategy |
|---|---|---|---|
| CodeRabbit | Pipeline + agentic | Dependency graph, multi-repo | Vector DB (LanceDB), learnings from past reviews |
| Greptile | Full agent (Claude Agent SDK) | Multi-hop call chain tracing | Semantic index of entire codebase |
| Augment | Context Engine | 400K+ file semantic index | Semantic search for cross-file relationships |
| Sourcery | Multi-angle specialists | Per-reviewer scope | Rules engine + LLM hybrid |
| Anthropic plugin | 4 parallel Sonnet agents | Git blame analysis | CLAUDE.md, confidence scoring |
| Qodo/PR-Agent | Single LLM per tool | PR-scoped | PR history + codebase context |

### xreview's Differentiation (unchanged)

1. Cross-model diversity (Codex + Claude Code)
2. Claude Code as intelligent verifier
3. Local-first, no index, no SaaS dependency
4. User in the loop (review-only mode, discussion, per-finding decisions)
