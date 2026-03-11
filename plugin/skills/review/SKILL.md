---
name: review
description: >
  AI-powered code review using Codex. Triggers after completing a plan or
  milestone to review changed files for bugs, security issues, and logic errors.
  Manages the full review lifecycle: discover, fix, verify, report.
allowed-tools: Bash(xreview *), Bash(${CLAUDE_PLUGIN_ROOT}/scripts/*), Bash(which *), Bash(go install *), AskUserQuestion, Read
argument-hint: [files-or-uncommitted]
---

# xreview - Agent-Native Code Review

## Step 0: Ensure xreview is installed

1. Check if xreview exists:
   Run: `which xreview`

2. If NOT found:
   a. Run the install script:
      `bash "${CLAUDE_PLUGIN_ROOT}/scripts/install.sh"`
   b. Verify: run `xreview version`
   c. If install fails, show the error to the user and stop.

3. If found, check version:
   Run: `xreview version`
   - Parse the XML output for the `outdated` attribute.
   - If outdated="true":
     Ask user: "xreview {current} is installed but {latest} is available. Update? (y/n)"
     If yes: run `bash "${CLAUDE_PLUGIN_ROOT}/scripts/install.sh"`

## Step 1: Ask user if they want a review

Ask the user: "Code review? (y/n)"
If no, stop. Do not proceed with any review steps.

## Step 2: Preflight check

Run: `xreview preflight`

Parse the XML output:
- If status="success": proceed to Step 3.
- If status="error": show the user the error message from the <error> tag.
  The error message is written for you to understand. Relay it to the user
  in natural language and suggest how to fix it. Stop.

## Step 3: Determine review targets and assemble context

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

## Step 4: Run review

Run: `xreview review --files <paths> --context "<structured context>"`
 or: `xreview review --git-uncommitted --context "<structured context>"`

## Step 5: Present findings and collect user decisions (Three-Party Consensus)

Parse the XML output.

If verdict is APPROVED (zero findings): tell the user "No issues found." Skip to Step 8.

For EACH finding, use AskUserQuestion to ask the user individually:
- Explain the finding in plain language (NOT raw XML)
- Provide YOUR (Claude Code) recommendation and reasoning
- Present options — **MUST always include "don't fix"**:
  (a) Fix as suggested — describe how you would fix it
  (b) Fix differently — ask user to explain their preferred approach
  (c) Don't fix — ask user for a brief reason (will be passed to codex for evaluation)

## Step 6: Apply fixes

After collecting all user decisions:
1. Apply the agreed fixes to the code.
2. Track what was fixed, what was dismissed, and the reasons for each.

## Step 7: Verify fixes (Three-Party Consensus Loop)

After applying fixes, run:
`xreview review --session <session-id> --message "<summary of what was fixed, dismissed, and reasons>"`

Parse the XML output:
- If codex confirms all fixes and accepts all dismissals → consensus reached. Proceed to Step 8.
- If codex disagrees with a dismissal or finds a fix incomplete or discovers new issues:
  Go back to Step 5 for the unresolved findings only. Present codex's response to the
  user via AskUserQuestion, explain the disagreement, and let the user decide again.
- This is a three-way conversation: codex reviews, Claude Code recommends, user decides.

Repeat until:
- All findings are resolved (fixed + dismissed with codex agreement) → proceed to Step 8.
- Maximum 5 rounds reached → inform the user of remaining unresolved items, proceed to Step 8.

## Step 8: Finalize

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
- Preflight only runs once per session. If a later command fails with a codex error,
  the error message will tell you what happened — no need to re-run preflight.
- This is a THREE-PARTY REVIEW: Codex (reviewer), you Claude Code (executor), and the
  user (decision maker). Every finding goes through AskUserQuestion — the user always
  has final say, including the option to not fix.
- Use --message to convey user decisions and your reasoning to codex. Codex is smart
  enough to reconsider when given good reasoning from the user.

## XML Schema Reference

See [reference.md](reference.md) for the complete XML schema documentation.
