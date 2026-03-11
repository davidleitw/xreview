---
name: xreview
description: >
  MANDATORY for ALL code review requests. When the user asks to "review", "code review",
  "check code", "找 bug", "review 程式碼", or any variation of reviewing code for bugs,
  security issues, or quality — you MUST use this skill. Do NOT read files and review
  them yourself. This skill delegates review to Codex (a separate AI reviewer) via the
  xreview CLI, enabling multi-round three-party review (Codex reviews, Claude Code fixes,
  user decides). Manages the full lifecycle: discover, fix, verify, report.
allowed-tools: Bash(xreview *), Bash(go install *), Bash(which *), AskUserQuestion, Read
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
  a. Check if Go is available: `which go`
  b. If yes: ask the user "xreview is not installed. Install it now? (y/n)"
     If yes: run `go install github.com/davidleitw/xreview@latest`, then re-run preflight.
  c. If no Go: tell the user to install Go or download the binary. Stop.

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

## Step 2.5: Fix Plan Gate (MANDATORY)

<CRITICAL>
- You MUST present ALL findings as a fix plan and get user approval BEFORE touching any code.
- Every finding MUST include: trigger, cascade impact, and ALL fix alternatives from the XML.
  Do NOT summarize to a single recommendation — the user needs options to decide.
- You MUST end the fix plan with AskUserQuestion. No exceptions.
- The xreview output includes `<agent-instructions>` after `</xreview-result>`. Follow them.
</CRITICAL>

Parse the XML output from Step 2.

If verdict is APPROVED (zero findings): tell the user "No issues found." Skip to Step 5.

### Build the Fix Plan

For EACH finding, present these fields (all available in the XML output):

1. **Header**: `### F-XXX: title (category/severity)` + `📍 file:line`
2. **Trigger**: the `<trigger>` content — copy it, don't rephrase or omit
3. **Impact**: what happens if exploited/triggered
4. **Cascade**: list every `<impact>` from `<cascade-impact>` — what else breaks if this is fixed
5. **Fix options**: ALL `<alternative>` entries from `<fix-alternatives>`, mark which is recommended.
   Always add a final option: "Don't fix — risk: _consequence_"

Low severity findings may use a shorter format but MUST still include fix options.

### Get User Approval

After listing ALL findings, use AskUserQuestion:

```
Fix plan for N findings above. How to proceed?
  A. Execute all recommended fixes
  B. Only fix high severity, skip the rest
  C. I want to adjust (tell me which findings to change — e.g. "F-003 skip, F-005 use option B")
```

Do NOT proceed until user responds.

## Step 3: Execute Fixes

Execute fixes strictly per the approved plan. No re-analysis, no ad-hoc decisions.

For each finding marked for fix:
1. Apply the chosen fix approach.
2. Briefly report what you did (one line per finding).

If user chose option C with adjustments, follow those exactly.
Skip any finding the user chose to not fix.

## Step 4: Summary + Verify

Present a summary table:

```
### Round N Summary

| ID    | Issue              | Action       | Detail                          |
|-------|--------------------|--------------|---------------------------------|
| F-001 | SQL injection      | Fixed (A)    | Changed to parameterized query  |
| F-002 | Unused error       | Not fixed    | User: acceptable for demo code  |
```

Then run verification with enhanced scope:

`xreview review --session <session-id> --message "<message>"`

The message MUST include:
- Which findings were fixed and how
- Which findings were dismissed and why
- Explicit instruction: "Re-review ALL modified files. Beyond verifying old findings, also check:
  1. Whether fixes introduced new security/logic issues
  2. Unhandled cascade impact between fixes
  3. Cross-layer consistency (if DB layer changed, are cache/handler layers in sync)"

Parse the result:
- All resolved → proceed to Step 5.
- Codex disagrees or finds new issues → present new/reopened findings with the same Fix Plan format (Step 2.5), get user approval, then fix.
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
  user (decision maker). You MUST present ALL findings as a Fix Plan and get user
  approval via AskUserQuestion BEFORE making any code changes. The user always has
  final say, including the option to not fix any finding.
- Use --message to convey user decisions and your reasoning to codex. Codex is smart
  enough to reconsider when given good reasoning from the user.

## XML Schema Reference

See [reference.md](reference.md) for the complete XML schema documentation.
