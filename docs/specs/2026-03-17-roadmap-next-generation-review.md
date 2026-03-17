# xreview Next-Generation Review — Roadmap & Design

Date: 2026-03-17
Status: Draft

**[中文版](2026-03-17-roadmap-next-generation-review.zh-TW.md)**

## Executive Summary

xreview's core value proposition is **cross-model code review**: Codex identifies issues, Claude Code independently verifies them, and the user decides. This three-party model eliminates same-model blind spots that plague single-model review tools (including Anthropic's own multi-agent code-review plugin, which runs 4 Sonnet instances that share identical training biases).

This document captures the strategic roadmap for xreview's next generation of features, informed by:
- Real-world feedback from production review sessions
- Competitive analysis of CodeRabbit, Greptile, Augment, Sourcery, Qodo, and Anthropic's official plugin
- Architecture discussions about how to leverage Claude Code's strength as an intelligent orchestrator

---

## The Problem: What xreview Can't Catch Today

### What We Can Catch vs. What We Can't

xreview is effective at catching **code-pattern issues** — problems visible within a single function or file: missing error handling, transactional safety gaps, schema migration problems. These are "code vs. correctness" issues.

But it consistently misses **semantic gap issues** — problems where the code works correctly but fails to communicate the developer's intent. These require understanding the design behind the code: how data structures relate across files, what a function name implies vs. what actually happens after it returns, whether a constant's name matches its semantic role in every usage site.

The gap is fundamental: Codex reviews code by analyzing what it **does**. Semantic gap issues are about what the code **should communicate** — a different kind of review entirely. Catching them requires cross-file structural understanding (symbol usage patterns, call chain tracing, data structure shape awareness) that single-function analysis cannot provide.

### The Chicken-and-Egg Problem

The `--context` flag could theoretically help — if the reviewer had the architectural context needed to spot semantic mismatches, it could catch them. But the developer who writes misleading code usually doesn't know they're doing it. Asking them to provide the context that reveals the problem is circular.

---

## Competitive Landscape

### What Others Do

| Tool | Architecture | Cross-File | Context Strategy |
|---|---|---|---|
| **CodeRabbit** | Pipeline + agentic hybrid | Dependency graph, multi-repo | Vector DB (LanceDB), 1:1 code-to-context ratio, learnings from past reviews |
| **Greptile** | Full agent (Claude Agent SDK) | Multi-hop investigation — recursively traces call chains | Semantic index of entire codebase |
| **Augment** | Context Engine | 400K+ file semantic index, full dependency graph | Semantic search for cross-file relationships |
| **Sourcery** | Multi-angle specialists | Per-reviewer scope | Rules engine + LLM hybrid |
| **Anthropic plugin** | 4 parallel Sonnet agents | Git blame analysis | CLAUDE.md, confidence scoring (threshold 80) |
| **Qodo/PR-Agent** | Single LLM per tool | PR-scoped | PR history + codebase context |

### xreview's Differentiation

1. **Cross-model diversity** — Codex and Claude have different training data, different attention patterns, different blind spots. This is not a bug; it's the core feature. Same-model multi-agent (Anthropic's 4 Sonnets) can't replicate this.

2. **Claude Code as intelligent verifier** — Not just a pass-through. Claude Code independently reads code, confirms or challenges findings, filters false positives. No other tool has a verification layer this strong.

3. **Local-first, no index** — No vector DB, no dependency graph infrastructure, no SaaS dependency. Uses Claude Code's real-time code reading + Codex's sandbox file access. Zero setup cost beyond installing Codex CLI.

4. **User in the loop** — Review-only mode lets users discuss findings, challenge them, request specific fixes. Not a CI bot that dumps comments on a PR.

---

## Strategic Direction

### Core Thesis

> **Don't build a better Codex. Build a better orchestrator around Codex.**

Claude Code is extraordinarily capable at reading and understanding code. The gap is not in Codex's review ability — it's in the **context and strategy** we give it. The next generation of xreview should:

1. Let Claude Code **gather structural context** before review (it's mechanical work, not review judgment)
2. Let Claude Code **design review strategy** based on what it sees in the code
3. Let xreview **dispatch focused Codex reviews** with rich context
4. Let Claude Code **cross-validate and synthesize** results

### Architecture Evolution

**Current (v0.8):**
```
User → Skill → xreview review → 1 Codex exec → findings → Claude Code verifies → User
```

**Next generation:**
```
User → Skill (teaches Claude Code how to think about review)
  │
  ├── Claude Code: determine scope (user instruction / commit / memory)
  ├── Claude Code: read code, build structural context
  ├── Claude Code: decide review strategy (single vs multi-angle)
  │
  ├── [single] xreview review --files ... --context-file ctx.json
  │
  ├── [multi]  xreview review --files ... --focus "angle 1" --context-file ctx.json --session-group grp &
  │            xreview review --files ... --focus "angle 2" --context-file ctx.json --session-group grp &
  │            xreview review --files ... --focus "angle 3" --context-file ctx.json --session-group grp &
  │            wait
  │            xreview merge --session-group grp
  │
  ├── Claude Code: verify + cross-validate findings
  └── User: discuss, decide, fix
```

**Key shift:** The skill becomes thicker — it describes conceptual review methodology, not just CLI invocation steps. Claude Code makes strategic decisions. xreview CLI stays a focused toolkit.

---

## Roadmap

### Phase 1: Context Engineering (Foundation)

**Goal:** Give Codex the structural context it needs to catch semantic issues, without building indexes or parsers.

#### 1a. `--context-file <path>` flag

Replace the `--context` string (which has length limits) with a file-based approach. Claude Code writes a structured JSON/YAML file before calling xreview; xreview injects its contents into the Codex prompt's context section.

```bash
xreview review --files handler.go,repo.go --context-file .xreview/context.json
```

Context file structure (prepared by Claude Code):

```json
{
  "project_summary": "HTTP API with layered architecture: Handler → Service → Repository → PostgreSQL",

  "key_data_structures": {
    "OrderCache": "map<string, []Order> — per-customer order lists",
    "GlobalOrderList": "[]Order — flat list across all customers"
  },

  "symbol_cross_references": {
    "kMaxCacheSize": [
      {"file": "cache.go:15", "usage": "definition, value=1000"},
      {"file": "handler.go:42", "usage": "len(globalList) < k — flat list, per-system semantic"},
      {"file": "handler.go:67", "usage": "len(customerOrders) < k — per-customer list, per-element semantic"},
      {"file": "repo.go:120", "usage": "SQL LIMIT clause — per-query semantic"}
    ]
  },

  "lifecycles": [
    "Order: create → validate → enqueue → process → ship → confirm → archive"
  ],

  "review_hints": [
    "OrderCache uses per-customer lists (map of slices), unlike GlobalOrderList which is flat",
    "markProcessed() only dequeues from the work queue, order continues through shipping pipeline"
  ]
}
```

**Why Claude Code prepares this, not the user:** Claude Code can mechanically grep symbol usages, trace call chains, and read struct definitions. It doesn't need to judge whether something is wrong — it just extracts structural facts. The chicken-and-egg problem is avoided because this is observation, not diagnosis.

**CLI changes:**
- New flag: `--context-file <path>` — reads file, injects into prompt context section
- `--context` remains for backward compatibility (short inline context)
- Both can be used together; context-file content appears first

#### 1b. `--focus <string>` flag

Tell Codex what angle to focus on for this review pass. Injected into the prompt's instruction section (not context).

```bash
xreview review --files handler.go,cache.go --focus "constant and enum semantic consistency across all usage sites"
```

**Distinction:**
- `--context-file` = **what the reviewer should know** (background knowledge)
- `--focus` = **what the reviewer should look for** (review instruction)

#### 1c. `--git-diff <ref>` scope mode

Review changes between refs, not just uncommitted changes or explicit file lists.

```bash
xreview review --git-diff HEAD~3          # last 3 commits
xreview review --git-diff origin/main     # changes since branching from main
xreview review --git-diff abc123..def456  # specific range
```

xreview runs `git diff <ref> --name-only` to get changed files, then proceeds as with `--files`.

#### 1d. Confidence scoring in Codex output schema

Add `confidence` field (0-100) to the finding schema. Codex assigns confidence based on how certain it is about the finding. xreview passes the score through; Claude Code uses it during verification (low-confidence findings get extra scrutiny).

```json
{
  "findings": [{
    "title": "Missing error handling in batch update",
    "severity": "medium",
    "confidence": 85,
    "category": "error-handling",
    ...
  }]
}
```

No automatic filtering by threshold — Claude Code decides what to do with confidence scores during verification.

### Phase 2: Multi-Angle Review

**Goal:** Let Claude Code dispatch multiple focused Codex reviews in parallel and merge results.

**Depends on:** Phase 1 (context-file and focus flags).

#### 2a. `--session-group <group-id>` flag

Tag a review session as part of a group. Multiple `xreview review` calls with the same group ID are logically linked.

```bash
GROUP="grp-$(date +%s)"

# Claude Code dispatches these in parallel via Agent tool
xreview review --files handler.go,cache.go \
  --focus "cross-file constant/enum semantic consistency" \
  --context-file ctx.json --session-group $GROUP

xreview review --files handler.go,service.go,repo.go \
  --focus "function naming vs actual behavior across request lifecycle" \
  --context-file ctx.json --session-group $GROUP

xreview review --files handler.go,repo.go \
  --focus "bugs, security, error handling" \
  --context-file ctx.json --session-group $GROUP
```

Each call creates its own session under `.xreview/sessions/` as usual, plus registers itself in `.xreview/groups/<group-id>.json`.

#### 2b. `xreview merge --session-group <group-id>`

Merge findings from all sessions in a group into a unified result.

```bash
xreview merge --session-group $GROUP
```

Merge logic:
1. Load all sessions in the group
2. Deduplicate: findings pointing to same file:line with similar descriptions → keep the one with higher confidence, note the other angle also flagged it
3. Unified numbering: F-001, F-002, ... across all angles
4. Preserve provenance: each finding records which angle/session discovered it
5. Output: same XML format as single review, compatible with existing skill flow

**Dedup strategy options** (specified via `--strategy`):
- `dedup` (default): merge overlapping findings, keep highest confidence
- `union`: keep all findings, mark duplicates but don't merge
- `intersect`: only keep findings flagged by 2+ angles (high-precision mode)

#### 2c. Skill update: review strategy decision

The skill teaches Claude Code when to use single vs multi-angle:

```markdown
## Review Strategy

Decide based on scope and complexity:

### Quick Review (single Codex)
- Few files (1-3), single concern
- User asked for specific check ("check for SQL injection")
- Time-sensitive, user wants fast feedback

→ xreview review --files <paths> [--focus <specific>] [--context-file <ctx>]

### Deep Review (multi-angle)
- Cross-subsystem changes
- Complex data flows or lifecycles
- User asked for thorough review
- You see code patterns that warrant cross-cutting analysis

→ Prepare context-file, decide angles, dispatch parallel reviews, merge

### How to decide angles
Read the code first. Look for:
- Constants/enums used across multiple files → angle: "semantic consistency"
- Multi-step lifecycles (create → process → complete) → angle: "lifecycle naming and state transitions"
- Complex data structures (maps of lists, nested containers) → angle: "data structure boundary checks"
- Always include a general angle: "bugs, security, error handling"
```

#### 2d. Multi-angle verification round

After fixes, Claude Code needs to verify across all angles. Two options:

- **Option A:** Resume each original session separately, merge again. Preserves per-angle Codex conversation context.
- **Option B:** Create a single new session for verification with all findings. Simpler but loses per-angle context.

Recommendation: **Option A** — the per-angle conversation context is valuable. Claude Code dispatches parallel `xreview review --session <id> --message "..."` for each angle's session, then `xreview merge` again.

### Phase 3: Enhanced Skill Intelligence

**Goal:** Make the skill's review methodology smarter without changing CLI.

#### 3a. Structural context gathering guide

Expand the skill to teach Claude Code how to prepare context-file:

```markdown
## Preparing Context (before calling xreview)

1. Read the target files
2. For each non-trivial symbol (constants, enums, key functions):
   - Grep all usage sites across the codebase
   - Note semantic differences in usage
3. For functions with lifecycle-implying names (done, complete, close, init, destroy):
   - Trace what happens after the function returns
   - Note if significant work remains
4. For key data structures:
   - Document their shape (flat list? map of lists? tree?)
   - Note where shape affects semantics
5. Write findings to context-file as structured JSON
```

This is mechanical work — Claude Code is not reviewing, just observing.

#### 3b. Review memory (cross-session learning)

When a user dismisses a finding type repeatedly, or when a false positive pattern emerges, store it:

```json
// .xreview/config.json
{
  "learned_rules": [
    {
      "pattern": "missing error handling on defer Close()",
      "action": "suppress",
      "reason": "project convention: defer Close() errors are intentionally ignored"
    }
  ]
}
```

xreview injects learned rules into the Codex prompt. Similar to CodeRabbit's learnings and Sourcery's feedback adaptation.

#### 3c. Language-aware review hints

Detect project language from file extensions, inject language-specific review patterns into the prompt:

```
Language: C++
- Check for aggregate initialization readability (return {}, designated initializers)
- Check RAII compliance: resources acquired in constructor must be released in destructor
- Check for implicit conversions and narrowing

Language: Go
- Check that all errors are handled or explicitly ignored with comment
- Check for goroutine leaks (unbounded goroutine spawning without lifecycle management)
- Check context.Context propagation through call chains
```

These are prompt additions, not code changes. Stored as data files in xreview, selected by language.

### Phase 4: New Review Modes

**Goal:** Support review targets beyond "files with code changes."

#### 4a. Design Plan Review (`xreview review-plan`)

Review an implementation plan / design doc before execution. Different prompt, different schema, different findings structure.

```bash
xreview review-plan --file docs/specs/feature-x-design.md --codebase-context ctx.json
```

Codex reads the plan + relevant existing code, checks for:
- Feasibility issues (plan assumes API that doesn't exist)
- Missing error handling in the plan
- Architectural conflicts with existing code
- Incomplete edge case coverage
- Scope concerns (plan tries to do too much)

Output schema differs from code review — findings are about the plan, not code locations.

#### 4b. Auto-Fix Mode (`--auto-fix`)

Fully autonomous review-and-fix cycle. Claude Code skips the review-only discussion phase and automatically applies recommended fixes through the three-party verify loop.

```bash
# In skill context, triggered by user saying "review and fix everything"
xreview review --files <paths> --context-file ctx.json
# Claude Code automatically fixes all high/critical findings
# Resumes for verification
# Repeats until clean or max rounds
```

This is a skill-level change (Claude Code's behavior), not a CLI change. The skill describes when auto-fix is appropriate (user explicitly requested it, vibe coding context) and when it's not (production code, team review context).

### Phase 5: Multi-Model Review (Future)

**Goal:** Support reviewers beyond Codex.

#### 5a. Reviewer abstraction

xreview currently has `internal/reviewer/reviewer.go` interface and `single.go` (Codex implementation). Extend to support:

- **Gemini reviewer** — Google's model via API, different blind spots from Codex
- **Local model reviewer** — Ollama/llama.cpp for offline use
- **Custom reviewer** — user-provided command that accepts prompt on stdin, returns JSON on stdout

```bash
xreview review --files <paths> --reviewer codex    # default
xreview review --files <paths> --reviewer gemini
xreview review --files <paths> --reviewer custom --reviewer-cmd "my-review-tool"
```

#### 5b. Second Opinion mode

Run the same code through multiple reviewers, each with its own session:

```bash
xreview review --files <paths> --reviewers codex,gemini --session-group grp
# Internally spawns two review processes
# xreview merge combines findings from different models
```

Cross-model findings (flagged by both Codex and Gemini independently) get boosted confidence. Single-model findings are still presented but marked as single-source.

---

## CLI Command Reference (Complete)

### Existing (unchanged)

| Command | Purpose |
|---|---|
| `xreview preflight` | Check environment readiness |
| `xreview version` | Show version + check for updates |
| `xreview self-update` | Update to latest release |
| `xreview report --session <id>` | Generate report |
| `xreview clean --session <id>` | Delete session data |

### Enhanced

| Command | Changes |
|---|---|
| `xreview review` | New flags: `--context-file`, `--focus`, `--git-diff`, `--session-group`, `--confidence-threshold` |

### New

| Command | Purpose | Phase |
|---|---|---|
| `xreview merge --session-group <id>` | Merge multi-angle findings | 2 |
| `xreview report --session-group <id>` | Report from merged results | 2 |
| `xreview clean --session-group <id>` | Clean all sessions in group | 2 |
| `xreview review-plan --file <path>` | Review a design/implementation plan | 4 |

### Full Flag Reference for `xreview review`

```
xreview review [flags]

Scope (mutually exclusive):
  --files <path,path,...>     Specific files to review
  --git-uncommitted           All uncommitted changes
  --git-diff <ref>            Changes relative to a git ref

Context:
  --context <string>          Inline context (short, backward compat)
  --context-file <path>       Structured context file (JSON/YAML)
  --focus <string>            Review angle / what to look for

Session:
  --session <id>              Resume existing session
  --session-group <group-id>  Tag session as part of a group
  --message <string>          Message for session resume

Output:
  --confidence-threshold <n>  Filter findings below confidence (default: 0, no filter)
  --json                      Raw JSON output instead of XML

Control:
  --timeout <seconds>         Codex execution timeout (default: 120)
  --full-rescan               Force re-read all files on resume
```

---

## Implementation Priority

| Phase | Effort | Impact | Dependencies |
|---|---|---|---|
| **1a: --context-file** | Small | High | None — unlocks rich context |
| **1b: --focus** | Small | High | None — unlocks multi-angle |
| **1c: --git-diff** | Small | Medium | None |
| **1d: Confidence scoring** | Small | Medium | Schema change |
| **2a: --session-group** | Medium | High | 1a, 1b |
| **2b: xreview merge** | Medium | High | 2a |
| **2c: Skill strategy update** | Medium | High | 2a, 2b |
| **2d: Multi-angle verification** | Medium | Medium | 2b |
| **3a: Context gathering guide** | Small (skill only) | High | 1a |
| **3b: Review memory** | Medium | Medium | — |
| **3c: Language-aware hints** | Small | Medium | — |
| **4a: review-plan** | Large | Medium | New prompt + schema |
| **4b: Auto-fix mode** | Medium (skill only) | Medium | Stable review loop |
| **5a: Reviewer abstraction** | Large | High | Stable merge |
| **5b: Second opinion** | Large | High | 5a |

### Recommended execution order

```
Phase 1a + 1b (parallel) → 1c + 1d (parallel) → 3a (skill, can overlap)
  → Phase 2a → 2b → 2c + 2d (parallel)
  → Phase 3b + 3c (parallel, independent)
  → Phase 4a or 4b (based on demand)
  → Phase 5 (when multi-angle is stable)
```

---

## Design Principles

1. **xreview is the toolkit, Claude Code is the brain.** Don't put intelligence in the CLI. Give Claude Code better tools and better guidance (via skill). Let it make strategic decisions.

2. **Cross-model diversity is the moat.** Same-model multi-agent review has a ceiling. Different models see different things. Protect and expand this advantage.

3. **Context over cleverness.** Real-world review sessions show that Codex with the right context could have caught semantic issues it missed. Don't build static analysis — build better context pipelines.

4. **Incremental evolution.** Every phase is independently valuable. Phase 1 alone (context-file + focus) already enables better reviews without multi-angle complexity.

5. **Backward compatible.** Existing `xreview review --files ... --context "..."` works unchanged. New features are additive.

---

## Open Questions

1. **Context-file format:** JSON vs YAML vs Markdown? JSON is easiest to parse in Go; YAML is more readable; Markdown might be most natural for Claude Code to generate. Recommendation: support JSON and YAML, let Claude Code choose.

2. **Merge dedup algorithm:** How to detect that two findings from different angles describe the same issue? By file:line proximity? By semantic similarity of description? Start simple (same file, overlapping line range, >70% description overlap via fuzzy match), iterate based on real usage.

3. **Session-group storage:** Flat file `.xreview/groups/<id>.json` listing member session IDs? Or embed group info in each session? Recommendation: flat file — simpler, avoids modifying session schema.

4. **Skill thickness:** How much review methodology should the skill prescribe vs let Claude Code figure out? Too prescriptive → rigid, can't adapt. Too open → inconsistent behavior. Start prescriptive (explicit decision trees), relax as we learn what Claude Code handles well autonomously.

5. **Context-file size:** Claude Code preparing a thorough context-file for a large codebase could itself be expensive (many grep/read operations). Should the skill set guidelines for context depth (e.g., "trace symbols max 2 hops", "cap at 50 cross-references")? Likely yes — needs experimentation to find the right balance.
