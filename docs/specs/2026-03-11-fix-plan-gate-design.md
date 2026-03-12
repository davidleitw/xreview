# Fix Plan Gate: Enriched Findings + Mandatory User Approval

Date: 2026-03-11

## Problem Statement

In real-world usage, the current SKILL.md Step 3 designed Case A (auto-fix) and Case B (ask user) branches, but Claude Code classifies nearly all findings as Case A and fixes them without user input. Three root causes:

1. **No global view**: Findings are analyzed and fixed one-at-a-time interleaved, so the user never sees the full picture before fixes begin.
2. **No mandatory pause**: There is no forced gate requiring user approval of a fix plan before execution starts.
3. **Thin finding data**: Each finding only has `description` + `suggestion` + `code_snippet` — not enough for the user to make informed decisions about fix approaches, cascade risks, or whether to fix at all.

## Design

Two coordinated changes: enrich Codex output (Go code) and restructure the skill workflow (SKILL.md).

### Part 1: Enriched Finding Schema

Add three fields to the Codex output schema so that Codex produces richer analysis per finding.

#### New fields

| Field | Type | Description |
|-------|------|-------------|
| `trigger` | `string` | Concrete trigger condition — specific input, scenario, or sequence that manifests the issue. Not abstract. Example: `"attacker sends id=' OR '1'='1 as taskID"` |
| `cascade_impact` | `array of string` | Other locations in the codebase affected if this finding is fixed. Each entry: `"file:function — what changes"`. Codex has repo access and can trace call chains. Empty array if no cascade. |
| `fix_alternatives` | `array of object` | 2-3 fix approaches. Each: `{ "label": "A", "description": "...", "effort": "minimal|moderate|large", "recommended": true/false }`. At least one must be `recommended: true`. |

All three fields apply to **all severities** — no conditional logic based on severity tier. This keeps the schema simple and lets the skill presentation layer decide how much detail to show.

#### Schema change (`internal/schema/review.json`)

Add to finding properties:

```json
"trigger": {
  "type": "string",
  "description": "Concrete trigger condition: specific input, scenario, or call sequence that manifests this issue"
},
"cascade_impact": {
  "type": "array",
  "items": { "type": "string" },
  "description": "Other codebase locations affected if this finding is fixed. Each entry: file:function — impact description. Empty array if none."
},
"fix_alternatives": {
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "label": { "type": "string" },
      "description": { "type": "string" },
      "effort": { "type": "string", "enum": ["minimal", "moderate", "large"] },
      "recommended": { "type": "boolean" }
    },
    "required": ["label", "description", "effort", "recommended"],
    "additionalProperties": false
  },
  "description": "2-3 fix approaches, at least one recommended"
}
```

Add `"trigger"`, `"cascade_impact"`, `"fix_alternatives"` to the `required` array.

#### Type change (`internal/session/types.go`)

```go
type Finding struct {
    // ... existing fields ...
    Trigger         string           `json:"trigger"`
    CascadeImpact   []string         `json:"cascade_impact"`
    FixAlternatives []FixAlternative `json:"fix_alternatives"`
}

type FixAlternative struct {
    Label       string `json:"label"`
    Description string `json:"description"`
    Effort      string `json:"effort"`
    Recommended bool   `json:"recommended"`
}
```

#### Prompt change (`internal/prompt/templates.go`)

Add guidance to `FirstRoundTemplate` instructing Codex to fill the new fields:

```
For each finding, you MUST provide:
- trigger: the concrete input, scenario, or call sequence that manifests the issue (not abstract)
- cascade_impact: other files/functions affected if this code is changed (trace the call chain). Empty array if none.
- fix_alternatives: 2-3 fix approaches with effort estimate. Mark exactly one as recommended.
```

Add the same fields to the inline schema in `ResumeTemplate`.

#### XML output change (`internal/formatter/xml.go`)

Add new child elements to `<finding>`:

```xml
<finding id="F-001" severity="high" category="security" status="open">
  <location file="store/db.go" line="19" />
  <description>...</description>
  <suggestion>...</suggestion>
  <code-snippet>...</code-snippet>
  <trigger>attacker sends id=' OR '1'='1 as taskID</trigger>
  <cascade-impact>
    <impact>handler/task.go:GetTaskHandler() — passes user input directly</impact>
    <impact>cache/task.go:GetCached() — bypasses DB validation on cache miss</impact>
  </cascade-impact>
  <fix-alternatives>
    <alternative label="A" effort="minimal" recommended="true">Change to db.QueryRow("...WHERE id = ?", taskID)</alternative>
    <alternative label="B" effort="large" recommended="false">Introduce ORM layer for all SQL construction</alternative>
  </fix-alternatives>
</finding>
```

#### Merge logic change (`internal/reviewer/single.go`)

`mergeFindings()` already updates existing findings by ID. New fields follow the same pattern: incoming values overwrite previous values.

### Part 2: Skill Workflow Restructure (SKILL.md)

#### New workflow

```
Step 0: Preflight           (unchanged)
Step 1: Determine targets   (unchanged)
Step 2: Run review          (unchanged)
Step 2.5: Fix Plan Gate     (NEW — mandatory)
Step 3: Execute fixes       (rewritten — follow approved plan)
Step 4: Verify              (enhanced — full re-review)
Step 5: Finalize            (unchanged)
```

#### Step 2.5: Fix Plan Gate

After parsing review XML, Claude Code MUST present ALL findings as a complete fix plan before touching any code. This is a **hard gate** — no fixes before user approval.

For each finding, present:

```
### F-001: SQL Injection (security/high)
📍 store/db.go:19 — GetTask()

**Trigger**: attacker sends id=' OR '1'='1 as taskID
**Root cause**: fmt.Sprintf concatenates user input directly into SQL
**Impact**: attacker can read, modify, or delete entire database
**Cascade**: if GetTask() changes, also affects:
  - handler/task.go:GetTaskHandler() — passes user input directly
  - cache/task.go:GetCached() — bypasses DB validation on cache miss

**Fix options**:
  A. (Recommended) Change to parameterized query — minimal effort
  B. Introduce ORM layer — large effort
  C. Don't fix — risk: full database compromise
```

Severity tiers guide presentation detail:
- **high/security**: Full analysis as above
- **medium**: Same structure, cascade and alternatives may be shorter
- **low**: Brief description, default to recommended fix, user can opt-out

After ALL findings are listed, use AskUserQuestion:

```
Fix plan for N findings above. How to proceed?
  A. Execute all recommended fixes
  B. Only fix high severity, skip the rest
  C. I want to adjust (tell me which findings to change — e.g. "F-003 skip, F-005 use option B")
```

#### Step 3: Execute Fixes (rewritten)

Execute fixes **strictly per the approved plan**. No re-analysis, no ad-hoc Case A/B decisions. For each finding marked for fix, apply the chosen approach and briefly report what was done.

If user chose option C and provided adjustments, follow those exactly.

#### Step 4: Verify (enhanced)

Change verification `--message` from "confirm these findings are fixed" to a full re-review scope:

```
xreview review --session <id> --message "Fixed: [list]. Dismissed: [list with reasons].
Please re-review ALL modified files. Beyond verifying old findings, also check:
1. Whether fixes introduced new security/logic issues
2. Unhandled cascade impact between fixes
3. Cross-layer consistency (if DB layer changed, are cache/handler layers in sync)"
```

This leverages the existing resume flow. Codex can return updated findings (status changes) plus new findings (status: open, new IDs) in the same response.

## Files to Change

| File | Change |
|------|--------|
| `internal/schema/review.json` | Add `trigger`, `cascade_impact`, `fix_alternatives` to schema |
| `internal/session/types.go` | Add new fields to `Finding` struct, add `FixAlternative` struct |
| `internal/prompt/templates.go` | Add field guidance to `FirstRoundTemplate`, add fields to `ResumeTemplate` inline schema |
| `internal/formatter/xml.go` | Add `<trigger>`, `<cascade-impact>`, `<fix-alternatives>` XML elements |
| `internal/reviewer/single.go` | Update `mergeFindings()` for new fields |
| `.claude/skills/xreview/SKILL.md` | Add Step 2.5, rewrite Step 3, enhance Step 4 |
| `.claude/skills/xreview/reference.md` | Document new XML elements |
| Tests | Update existing tests, add cases for new fields |

## Out of Scope

- Finding count limits (no cap on number of findings — revisit if output limit is hit)
- Separate "continue finding" flow (existing resume already supports new findings in verification rounds)
- Changes to preflight, report, clean, or self-update commands
- Changes to collector or codex runner
