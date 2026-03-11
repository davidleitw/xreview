---
name: review
description: >
  MANDATORY for ALL code review requests. When the user asks to "review", "code review",
  "check code", "找 bug", "review 程式碼", or any variation of reviewing code for bugs,
  security issues, or quality — you MUST use this skill. Do NOT read files and review
  them yourself. This skill delegates review to Codex (a separate AI reviewer) via the
  xreview CLI, enabling multi-round three-party review (Codex reviews, Claude Code fixes,
  user decides). Manages the full lifecycle: discover, fix, verify, report.
allowed-tools: Bash(xreview *), Bash(${CLAUDE_PLUGIN_ROOT}/scripts/*), Bash(which *), Bash(go install *), AskUserQuestion, Read
argument-hint: [files-or-uncommitted]
---

# xreview - Agent-Native Code Review

<CRITICAL>
You MUST use this skill for ANY code review task. NEVER review code by reading files yourself.
The entire point of xreview is to delegate review to Codex (a separate AI model) so you get
an independent second opinion. If you skip this skill and review code yourself, you defeat
the purpose — you're reviewing your own work instead of getting an external review.
</CRITICAL>

## Step 0: Preflight

Run: `xreview preflight`

This single command checks everything: xreview version, codex installation, API key.

Parse the XML output:
- If status="success": proceed to Step 1.
- If status="error": show the user the error message from the <error> tag.
  Relay it in natural language and suggest how to fix it. Stop.

If xreview itself is not found (`which xreview` fails):
  a. Run the install script:
     `bash "${CLAUDE_PLUGIN_ROOT}/scripts/install.sh"`
  b. Verify: run `xreview version`
  c. If install fails, show the error to the user and stop.

## Step 1: Determine review targets and assemble context

Based on the current task, determine which files to review:
- If you just completed a plan with specific files changed, use --files with those paths.
- If reviewing a whole directory, pass the directory path to --files (xreview expands it).
- If unsure which files changed, use --git-uncommitted.

Assemble a structured --context string describing the change:

```
【變更類型】feature | refactor | bugfix
【描述】簡述做了什麼
【預期行為】這段 code 應該達成什麼效果（refactor 則寫「行為應與修改前一致」）
```

## Step 2: Run review

Run: `xreview review --files <paths> --context "<structured context>"`
 or: `xreview review --git-uncommitted --context "<structured context>"`

## Step 3: Analyze findings and fix (Three-Party Consensus)

Parse the XML output.

If verdict is APPROVED (zero findings): tell the user "No issues found." Skip to Step 5.

For EACH finding, process it **one at a time** in this order:

### 1. Analyze

For every finding, explain in plain language:
- **What**: what the issue is (in one sentence)
- **Where**: file, line, function name
- **Trigger**: under what conditions this bug manifests
- **Root cause**: why the code is wrong
- **Impact**: what happens if not fixed (data loss, security breach, crash, etc.)

Format example:
```
**F-001: SQL Injection** (security/high)
📍 store/db.go:34 — FindUser()

Trigger: user sends malicious string via /user?name=' OR '1'='1
Root cause: fmt.Sprintf concatenates user input directly into SQL query
Impact: attacker can read, modify, or delete any data in the database
```

### 2. Decide

Assess whether there is one obvious fix or multiple valid approaches:

**Case A — Single obvious fix:**
State the fix, apply it immediately, and briefly report what you did.
Example: "→ Fix: changed to parameterized query `db.Query("...WHERE name = ?", name)`"

**Case B — Multiple valid approaches or ambiguous trade-off:**
Use AskUserQuestion with concrete options. Put your recommended option first
with "(Recommended)". **MUST always include a "Don't fix" option.**
Example options:
- "Use parameterized query (Recommended)" — why
- "Use an ORM layer" — why
- "Don't fix" — skip this finding

### 3. Fix

After deciding (Case A: immediately; Case B: after user responds), apply the fix
**before** moving to the next finding. If user chose "Don't fix", record the reason.

## Step 4: Summary + Verify

After ALL findings are processed, present a summary table:

```
### Round N Summary

| ID    | Issue              | Action       | Detail                          |
|-------|--------------------|--------------|---------------------------------|
| F-001 | SQL injection      | Fixed        | Changed to parameterized query  |
| F-002 | Unused error       | Not fixed    | User: acceptable for demo code  |
```

Then run verification:
`xreview review --session <session-id> --message "<summary of what was fixed, dismissed, and reasons>"`

Parse the result:
- All resolved → proceed to Step 5.
- Codex disagrees or finds new issues → go back to Step 3 for unresolved findings only.
- Maximum 5 rounds → inform user of remaining items, proceed to Step 5.

## Step 5: Finalize

Run: `xreview report --session <session-id>`

Tell the user: "Review complete. Report saved to {report-path}."
Provide a brief summary of the final finding statuses.

Ask the user (AskUserQuestion): "Clean up the review session? (y/n)"
If yes: run `xreview clean --session <session-id>` to remove session data.

## Important notes

- ALWAYS present findings in plain language, NOT raw XML.
- The <error> messages from xreview are written for you (an AI agent). Use them
  to understand what went wrong and explain it to the user naturally.
- Do NOT read or write .xreview/ directory files directly. Use only xreview CLI commands.
- The session ID is in the XML output's session attribute. Track it for resume calls.
- If any xreview command fails, show the error to the user and ask how to proceed.
- This is a THREE-PARTY REVIEW: Codex (reviewer), you Claude Code (executor), and the
  user (decision maker). For straightforward fixes, act directly to reduce noise.
  For ambiguous cases, the user always has final say via AskUserQuestion,
  including the option to not fix.
- Use --message to convey user decisions and your reasoning to codex. Codex is smart
  enough to reconsider when given good reasoning from the user.

## XML Schema Reference

See [reference.md](reference.md) for the complete XML schema documentation.
