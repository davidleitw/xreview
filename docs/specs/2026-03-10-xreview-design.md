# xreview Design Spec

**Date:** 2026-03-10
**Status:** Draft
**Language:** Go
**Repository:** github.com/davidleitw/xreview

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [Installation & Distribution](#3-installation--distribution)
4. [CLI Interface](#4-cli-interface)
5. [Session State Machine](#5-session-state-machine)
6. [Data Formats](#6-data-formats)
7. [Codex Integration](#7-codex-integration)
8. [Skill Design (Claude Code Integration)](#8-skill-design-claude-code-integration)
9. [Error Handling](#9-error-handling)
10. [Testing Strategy](#10-testing-strategy)
11. [Project Structure (Go)](#11-project-structure-go)
12. [Future: Multi-Agent Review (Option B)](#12-future-multi-agent-review-option-b)
13. [Open Questions](#13-open-questions)
14. [Appendix: Claude Code Skill & Plugin Reference](#14-appendix-claude-code-skill--plugin-reference)

---

## 1. Overview

xreview is an **Agent-Native Code Review Engine** — a Go CLI tool that orchestrates
code review between Claude Code (the host agent that writes code) and Codex (the
review agent that analyzes code).

### Core Loop

```
Discover issues --> Guide fixes --> Verify corrections --> Generate report
```

A human developer uses Claude Code to implement features. After completing a task,
xreview (triggered via a Claude Code skill) collects the changed files, sends them
to a local Codex process for review, parses the structured findings, and returns
them to Claude Code. Claude Code then presents each finding to the user via
`AskUserQuestion`, providing its own recommendation and always offering the option
to not fix. After the user decides on each finding, Claude Code applies fixes and
calls xreview again to verify. This loop repeats until all three parties — the
**user (decision maker)**, **Claude Code (executor)**, and **Codex (reviewer)** —
reach consensus, or the maximum round limit is reached.

### What xreview IS

- An orchestrator between two AI agents (Claude Code and Codex)
- A session manager that tracks review state across multiple rounds
- A format translator (codex JSON -> internal state -> XML for Claude Code)

### What xreview is NOT

- Not a GitHub bot or PR review service
- Not a direct user-facing tool (primary interface is through Claude Code skill)
- Not an LLM wrapper — it does NOT hold API keys or call LLM APIs directly
- Not a code formatter or linter — it delegates all analysis to Codex

### Key Design Principle: Data-Presentation Decoupling

xreview enforces strict separation between internal state and external presentation:

| Layer | Format | Purpose |
|---|---|---|
| **Internal state** | JSON schemas in `.xreview/sessions/` | Persistent, machine-readable, powers state machine |
| **AI-to-AI output** | XML-tagged format on stdout | Consumed by Claude Code skill for structured parsing |
| **Human-facing output** | Natural language | Claude Code skill translates XML into plain language |

The xreview binary never speaks directly to the user. It only outputs structured
XML on stdout. The Claude Code skill is responsible for all human interaction.

---

## 2. Architecture

### System Diagram

```
Human User
    |
    | (natural language conversation)
    |
Claude Code (Host Agent)
    |
    | SKILL.md defines the workflow.
    | Skill uses AskUserQuestion to interact with the user.
    | Skill invokes xreview CLI via Bash tool.
    | Skill translates XML output into human-readable summaries.
    |
xreview CLI (Go binary)
    |
    | Manages sessions in .xreview/ directory.
    | Collects file contents and assembles prompts.
    | Spawns codex processes and parses their output.
    | Outputs structured XML on stdout.
    |
codex CLI (local process)
    |
    | Performs LLM-powered code analysis.
    | Supports session resume for multi-round context retention.
    | Returns structured JSON findings.
```

### Responsibility Boundaries

| Layer | Does | Does NOT |
|---|---|---|
| **Claude Code Skill** | Trigger review, ask user yes/no, present findings in plain language, fix code, decide when to stop | Parse .xreview/ files, call codex, decide what to fix (user decides) |
| **xreview binary** | Manage sessions, collect files, assemble codex prompts, parse codex output, format XML, track finding state | Talk to user, hold API keys, make review judgments, modify source code |
| **codex CLI** | Analyze code, identify issues, verify fixes, maintain conversation memory via session resume | Manage state, format output for Claude Code, track findings across rounds |

### Data Flow (Single Round)

```
1. Skill determines changed files + context from current task
2. Skill calls: xreview review --files a.go,b.go --context "..."
3. xreview reads file contents from disk
4. xreview assembles prompt with file contents + review instructions + JSON schema
5. xreview spawns: codex exec -m <model> "<prompt>"
6. codex analyzes code, returns JSON findings on stdout
7. xreview parses JSON, assigns finding IDs (F001, F002, ...)
8. xreview writes session state to .xreview/sessions/<id>/
9. xreview outputs XML on stdout
10. Skill reads XML, translates to: "Found 2 issues: ..."
11. User decides what to fix
```

### Data Flow (Resume Round)

```
1. Claude Code fixes code based on user decision
2. Skill calls: xreview review --session <id> --message "Fixed F001, F002 is false positive"
3. xreview reads session state + current file contents
4. xreview assembles resume prompt with previous findings + status + updated files
5. xreview spawns: codex exec --resume <codex-session-id> "<prompt>"
6. codex verifies fixes with full context from previous round
7. xreview parses response, updates finding statuses
8. xreview outputs XML with updated findings
9. Skill presents results: "F001 verified fixed, F002 dismissed"
10. Loop or finish
```

---

## 3. Installation & Distribution

### Day 1: `go install`

Primary installation method. Target users are developers who likely have Go installed.

```bash
go install github.com/davidleitw/xreview@latest
```

### Future: GitHub Releases (binary download)

For users without Go. The Skill can detect OS/arch and download the correct binary.

```bash
# Example: skill-generated install command
curl -sSL https://github.com/davidleitw/xreview/releases/latest/download/xreview-linux-amd64 -o /usr/local/bin/xreview
chmod +x /usr/local/bin/xreview
```

### Skill-Driven Auto-Install

The Claude Code skill handles installation as Step 0 of its workflow. The user
never needs to manually install xreview — the skill does it for them.

**Skill install logic (pseudocode in SKILL.md):**

```
Step 0: Ensure xreview is installed

1. Run: which xreview
2. If not found:
   a. Run: which go
   b. If Go available:
      - Ask user: "xreview is not installed. Install it now? (y/n)"
      - If yes: go install github.com/davidleitw/xreview@latest
      - Verify: xreview version
   c. If Go not available:
      - Tell user: "xreview requires Go to install. Please install Go first,
        or download the binary from https://github.com/davidleitw/xreview/releases"
      - Stop.
3. If found, check version:
   - Run: xreview version
   - If output indicates outdated:
     - Ask user: "xreview v0.1.0 installed, v0.2.0 available. Update? (y/n)"
     - If yes: xreview self-update
```

### Version Check & Self-Update

```bash
# Check current version and latest available version
xreview version

# Update to latest version
xreview self-update
```

**`xreview version` output:**

```xml
<xreview-result status="success" action="version">
  <version current="0.1.0" latest="0.2.0" outdated="true"
           update-command="go install github.com/davidleitw/xreview@latest" />
</xreview-result>
```

```xml
<xreview-result status="success" action="version">
  <version current="0.2.0" latest="0.2.0" outdated="false" />
</xreview-result>
```

**Latest version detection:** HTTP GET to GitHub API:
`https://api.github.com/repos/davidleitw/xreview/releases/latest`
Parse the `tag_name` field. Cache the result for 1 hour to avoid rate limiting.

**`xreview self-update` behavior:**

1. Check if Go is available
2. If yes: run `go install github.com/davidleitw/xreview@latest`
3. If no: print download URL for the latest binary release
4. Verify the new version after update

**`xreview self-update` output (success):**

```xml
<xreview-result status="success" action="self-update">
  <update from="0.1.0" to="0.2.0" />
</xreview-result>
```

**`xreview self-update` output (already latest):**

```xml
<xreview-result status="success" action="self-update">
  <update from="0.2.0" to="0.2.0" already-latest="true" />
</xreview-result>
```

### Distribution: Claude Code Plugin

When xreview matures, it is packaged as a Claude Code plugin for single-command
installation. The plugin bundles the skill, hooks, and an install script together.

#### Plugin Directory Structure

```
xreview-plugin/
  .claude-plugin/
    plugin.json                    # Plugin manifest (REQUIRED)
  skills/
    review/                        # Skill: invoked as /xreview:review
      SKILL.md
      reference.md
  hooks/
    hooks.json                     # Hook definitions
  scripts/
    install-binary.sh              # Detects OS/arch, installs xreview binary
    check-binary.sh                # Version check helper
```

**CRITICAL**: `.claude-plugin/` contains ONLY `plugin.json`. All other directories
(`skills/`, `hooks/`, `scripts/`) must be at the plugin root, NOT inside `.claude-plugin/`.

#### plugin.json

```json
{
  "name": "xreview",
  "version": "0.1.0",
  "description": "Agent-native code review engine powered by Codex",
  "author": {
    "name": "davidleitw",
    "url": "https://github.com/davidleitw"
  },
  "repository": "https://github.com/davidleitw/xreview-plugin",
  "license": "MIT",
  "keywords": ["code-review", "codex", "agent"],
  "skills": "./skills/",
  "hooks": "./hooks/hooks.json"
}
```

Key fields:
- `name`: Becomes the namespace prefix. Skills are invoked as `/xreview:<skill-name>`
- `version`: Semver. Plugin manager uses this for update checks
- `skills`: Path to skill directories (relative, must start with `./`)
- `hooks`: Path to hook config (relative, must start with `./`)

#### Namespacing

Plugin skills are automatically namespaced:

| Skill location | Invocation |
|---|---|
| `skills/review/SKILL.md` | `/xreview:review` |
| `skills/report/SKILL.md` | `/xreview:report` (if we split into separate skills) |

Cannot use short names like `/review` — plugin namespace is always required.

#### Hooks (Auto-Trigger After Plan Completion)

`hooks/hooks.json`:

```json
{
  "hooks": {
    "Stop": [
      {
        "matcher": "auto",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/scripts/check-review-trigger.sh"
          }
        ]
      }
    ]
  }
}
```

The `Stop` hook fires when Claude Code finishes responding. The script can check
whether the last task was a plan completion and output a reminder message. However,
for Day 1, the simpler approach is to rely on the skill description to prompt
Claude Code to suggest `/xreview:review` at the right time.

`${CLAUDE_PLUGIN_ROOT}` resolves to the plugin's absolute directory path at runtime.

#### Installation & Management

```bash
# Install from marketplace (future)
claude plugin install xreview

# Install from git URL
claude plugin install github.com/davidleitw/xreview-plugin

# Local development testing
claude --plugin-dir ./xreview-plugin

# Manage
claude plugin enable xreview
claude plugin disable xreview
claude plugin update xreview
claude plugin uninstall xreview
```

Scopes for installation:
- `--scope user` (default): personal, all projects
- `--scope project`: shared via git (added to `.claude/plugins/`)
- `--scope local`: gitignored, this machine only

#### Binary Installation via Plugin

The skill's Step 0 uses `${CLAUDE_PLUGIN_ROOT}/scripts/install-binary.sh` to install
the xreview Go binary. The script handles OS/arch detection:

```bash
#!/bin/bash
# scripts/install-binary.sh
set -euo pipefail

VERSION="${1:-latest}"

# Check if already installed and up to date
if command -v xreview &>/dev/null; then
    CURRENT=$(xreview version --short 2>/dev/null || echo "unknown")
    if [ "$VERSION" = "latest" ] || [ "$CURRENT" = "$VERSION" ]; then
        echo "xreview $CURRENT is already installed"
        exit 0
    fi
fi

# Try go install first
if command -v go &>/dev/null; then
    go install "github.com/davidleitw/xreview@$VERSION"
    echo "installed via go install"
    exit 0
fi

# Fallback: download binary from GitHub Releases
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
esac

URL="https://github.com/davidleitw/xreview/releases/download/v${VERSION}/xreview-${OS}-${ARCH}"
DEST="${GOPATH:-$HOME/go}/bin/xreview"
curl -sSL "$URL" -o "$DEST" && chmod +x "$DEST"
echo "installed binary to $DEST"
```

#### Version Consistency

Plugin version and binary version are tracked independently but should be compatible:
- `plugin.json` version: plugin structure version (skill format, hook config)
- `xreview version`: binary version (CLI behavior, codex integration)

The skill's Step 0 checks binary version. If the binary is too old for the current
plugin's skill expectations, the skill prompts an update.

### Day 1 vs Plugin Timeline

| Phase | Distribution | Skill location | Install method |
|---|---|---|---|
| **Day 1** | Project-level skill + manual binary install | `.claude/skills/xreview/SKILL.md` | `go install` |
| **Day 2** | Plugin package | `xreview-plugin/skills/review/SKILL.md` | `claude plugin install` |

Day 1 uses a project-level skill. The plugin packaging is a **separate repo**
(`xreview-plugin`) that wraps the skill + hooks + install scripts. The Go binary
repo (`xreview`) remains independent.

---

## 4. CLI Interface

Six commands total. Each is a single invocation — xreview is stateless per execution,
session persistence lives in `.xreview/` on disk.

### Command Overview

| Command | Purpose | When Skill uses it |
|---|---|---|
| `xreview version` | Show version, check for updates | Step 0: install/update check |
| `xreview self-update` | Update to latest version | Step 0: if outdated |
| `xreview preflight` | Verify codex environment | Step 1: before first review |
| `xreview review` | Run new review or continue session | Step 2+: core review loop |
| `xreview report` | Generate final report | Last step: after all findings resolved |
| `xreview clean` | Delete session data | After user confirms review is done |

### `xreview version`

```bash
xreview version
```

No flags. Outputs current version and latest available version. See Section 3 for
output format.

### `xreview self-update`

```bash
xreview self-update
```

No flags. Updates the binary to the latest version. See Section 3 for details.

### `xreview preflight`

Validates that the codex environment is ready before attempting a review.

```bash
xreview preflight
```

**Checks performed (in order, fail-fast):**

| # | Check | Method | Failure code |
|---|---|---|---|
| 1 | codex binary exists | `exec.LookPath("codex")` | `CODEX_NOT_FOUND` |
| 2 | codex is authenticated | Run `codex auth status` or equivalent | `CODEX_NOT_AUTHENTICATED` |
| 3 | codex can respond | Run a minimal prompt: `codex exec "respond with OK"` | `CODEX_UNRESPONSIVE` |

Checks run sequentially. If check N fails, checks N+1... are skipped (no point
testing auth if codex isn't installed).

**Output (all checks pass):**

```xml
<xreview-result status="success" action="preflight">
  <checks>
    <check name="codex_installed" passed="true" detail="codex found at /usr/local/bin/codex" />
    <check name="codex_authenticated" passed="true" detail="authenticated as user@example.com" />
    <check name="codex_responsive" passed="true" detail="codex responded in 2.3s" />
  </checks>
</xreview-result>
```

**Output (check fails):**

```xml
<xreview-result status="error" action="preflight">
  <error code="CODEX_NOT_AUTHENTICATED">
    codex is installed but not logged in. Please ask the user to run: codex login
  </error>
  <checks>
    <check name="codex_installed" passed="true" detail="codex found at /usr/local/bin/codex" />
    <check name="codex_authenticated" passed="false" />
  </checks>
</xreview-result>
```

### `xreview review`

The core command. Starts a new review or continues an existing session.

```bash
# --- New review ---

# Single file
xreview review --files src/auth.go \
               --context "【變更類型】feature【描述】新增 JWT 認證【預期行為】登入回傳 15 分鐘過期 token"

# Multiple files
xreview review --files src/auth.go,src/middleware.go \
               --context "【變更類型】feature【描述】JWT 認證系統【預期行為】middleware 驗證每個 request 的 token"

# Directory (xreview auto-expands, respects ignore_patterns)
xreview review --files src/ \
               --context "【變更類型】refactor【描述】重構 auth 模組【預期行為】行為應與修改前一致"

# Mixed files and directories
xreview review --files src/auth.go,internal/ \
               --context "..."

# Review all uncommitted changes (staged + unstaged)
xreview review --git-uncommitted \
               --context "【變更類型】bugfix【描述】修復登入逾時問題【預期行為】token 過期後正確回傳 401"

# --- Continue existing session ---

# Resume: codex retains memory from previous round
xreview review --session xr-20260310-a1b2c3 \
               --message "Fixed the JWT expiration issue. The error wrapping one is a false positive."

# Full rescan: new codex session, but compare findings against previous round
xreview review --session xr-20260310-a1b2c3 \
               --message "Major refactor, please re-review everything" \
               --full-rescan
```

**Flags (new review):**

| Flag | Required | Description |
|---|---|---|
| `--files <path1,path2,...>` | One of `--files` or `--git-uncommitted` | Comma-separated file paths or directory paths. Directories are recursively expanded, respecting `ignore_patterns` in config. |
| `--git-uncommitted` | One of `--files` or `--git-uncommitted` | Collect all uncommitted changes (staged + unstaged tracked files) |
| `--context <text>` | No (recommended) | Structured context from Claude Code describing the change. Skill guides Claude Code to include: change type (feature/refactor/bugfix), description, and expected behavior. Passed verbatim to codex prompt. |
| `--timeout <seconds>` | No (default: 180) | Maximum time to wait for codex response |

**Flags (continue session):**

| Flag | Required | Description |
|---|---|---|
| `--session <id>` | Yes | Session ID from a previous `xreview review` |
| `--message <text>` | No (recommended) | Natural language description of what was fixed/dismissed. Passed to codex as context. |
| `--full-rescan` | No | Start a fresh codex session instead of resuming. xreview still compares new findings against previous round. |
| `--timeout <seconds>` | No (default: 180) | Maximum time to wait for codex response |

**Validation rules:**

- `--files` and `--git-uncommitted` are mutually exclusive
- `--files` and `--git-uncommitted` cannot be used with `--session`
- `--message` and `--full-rescan` require `--session`
- All paths in `--files` must exist on disk (files or directories)
- Directories in `--files` are recursively expanded; files matching `ignore_patterns` are excluded
- If a directory expands to zero files after filtering, emit `NO_TARGETS` error
- `--git-uncommitted` requires being inside a git repository
- `--session` ID must exist in `.xreview/sessions/`

**Behavior matrix:**

| Scenario | xreview session | codex session | Finding comparison |
|---|---|---|---|
| New review (no `--session`) | Create new | Create new | N/A (first round) |
| Resume (`--session`) | Read existing | Resume stored `codex_session_id` | Compare against previous round |
| Full rescan (`--session` + `--full-rescan`) | Read existing, keep history | Create new (ignore stored codex_session_id) | Diff new findings against previous round, annotate what's new/resolved |

**Output:** See Section 6.3 for XML output format.

### `xreview report`

Generates a human-readable Markdown report from a session.

```bash
xreview report --session xr-20260310-a1b2c3
```

**Flags:**

| Flag | Required | Description |
|---|---|---|
| `--session <id>` | Yes | Session ID to generate report for |

**Behavior:**

1. Read session state and all round data
2. Generate `report.md` in `.xreview/sessions/<id>/report.md`
3. Output XML confirmation on stdout with the report path

**Output:**

```xml
<xreview-result status="success" action="report" session="xr-20260310-a1b2c3">
  <report path=".xreview/sessions/xr-20260310-a1b2c3/report.md" />
  <summary rounds="3" total-findings="5" fixed="3" dismissed="1" open="1" />
</xreview-result>
```

**report.md content (example):**

```markdown
# Code Review Report

**Session:** xr-20260310-a1b2c3
**Date:** 2026-03-10
**Rounds:** 3
**Files Reviewed:** src/auth.go, src/middleware.go

## Summary

| Status | Count |
|--------|-------|
| Fixed | 3 |
| Dismissed | 1 |
| Open | 1 |
| **Total** | **5** |

## Findings

### [F001] HIGH - security | src/auth.go:42 | FIXED
JWT token is not checked for expiration.
- **Round 1:** Identified
- **Round 2:** User fixed, verified correct

### [F002] MEDIUM - logic | src/middleware.go:15 | DISMISSED
Error returned without context wrapping.
- **Round 1:** Identified
- **Round 2:** User dismissed (error is re-wrapped at caller level)

...
```

### `xreview clean`

Removes a session's data from `.xreview/sessions/`.

```bash
xreview clean --session xr-20260310-a1b2c3
```

**Flags:**

| Flag | Required | Description |
|---|---|---|
| `--session <id>` | Yes | Session ID to delete |

**Behavior:**

1. Verify session exists
2. Delete the entire `.xreview/sessions/<id>/` directory
3. Output confirmation

**Output:**

```xml
<xreview-result status="success" action="clean" session="xr-20260310-a1b2c3">
  <cleaned session="xr-20260310-a1b2c3" />
</xreview-result>
```

The skill calls this after the user confirms the review is complete and they don't
need the session data anymore. Session data includes raw codex output and findings
history, which can accumulate over time.

### Global Flags

These flags apply to all commands:

| Flag | Description |
|---|---|
| `--workdir <path>` | Override working directory (default: current directory). `.xreview/` is created here. |
| `--verbose` | Print debug information to stderr (does not affect stdout XML output) |
| `--json` | Output raw JSON instead of XML on stdout (for debugging or alternative integrations) |

---

## 5. Session State Machine

### Directory Layout

```
.xreview/
  config.json                          # Optional: project-level xreview configuration
  sessions/
    xr-20260310-a1b2c3/
      session.json                     # Session state + findings (single file, updated each round)
```

**Simplified design:** Each session is a single `session.json` file that gets updated
in-place each round. No per-round snapshots, no separate findings file. Codex retains
full conversation memory via `--resume`, so xreview does not need to replay history
from disk.

### Session ID Format

```
xr-<YYYYMMDD>-<random6hex>

Example: xr-20260310-a1b2c3
```

The date prefix makes sessions easy to sort chronologically. The random suffix
prevents collisions when multiple reviews happen on the same day.

### config.json (Optional Project-Level Config)

```json
{
  "codex_model": "gpt-5.4",
  "default_timeout": 180,
  "default_context": "",
  "ignore_patterns": [
    "**/*_test.go",
    "**/vendor/**",
    "**/*.generated.go"
  ]
}
```

If `.xreview/config.json` exists, xreview reads it and applies defaults. CLI flags
override config values. If it does not exist, built-in defaults are used.

### session.json

A single file that contains both session metadata and current finding states.
Updated in-place each round.

```json
{
  "session_id": "xr-20260310-a1b2c3",
  "xreview_version": "0.1.0",
  "created_at": "2026-03-10T14:30:00Z",
  "updated_at": "2026-03-10T14:45:00Z",
  "status": "in_review",
  "round": 2,
  "codex_session_id": "cs-xxxxxx",
  "codex_model": "gpt-5.4",
  "context": "【變更類型】feature【描述】JWT 認證系統【預期行為】登入回傳 15 分鐘過期 token",
  "targets": [
    "src/auth.go",
    "src/middleware.go"
  ],
  "target_mode": "files",
  "findings": [
    {
      "id": "F001",
      "severity": "high",
      "category": "security",
      "status": "fixed",
      "file": "src/auth.go",
      "line": 42,
      "description": "JWT token is not checked for expiration.",
      "suggestion": "Add exp claim validation after jwt.Parse."
    },
    {
      "id": "F002",
      "severity": "medium",
      "category": "logic",
      "status": "dismissed",
      "file": "src/middleware.go",
      "line": 15,
      "description": "Error returned without context wrapping.",
      "suggestion": "Use fmt.Errorf(\"middleware auth: %w\", err)."
    }
  ]
}
```

**Field descriptions:**

| Field | Type | Description |
|---|---|---|
| `session_id` | string | Unique xreview session ID |
| `xreview_version` | string | xreview version that created this session |
| `created_at` | ISO 8601 | When the session was created |
| `updated_at` | ISO 8601 | When the session was last modified |
| `status` | enum | Current session state (see state machine below) |
| `round` | int | Current round number (1-indexed) |
| `codex_session_id` | string | Codex session ID for resume (null if not yet called) |
| `codex_model` | string | Which codex model was used |
| `context` | string | Structured context from Claude Code (change type, description, expected behavior) |
| `targets` | []string | Files being reviewed |
| `target_mode` | string | How targets were specified: `"files"` or `"git-uncommitted"` |
| `findings` | []Finding | Current finding states, updated in-place each round |

### Status Transitions

```
                    +-----------+
                    |           |
                    v           |
initialized --> in_review --> verifying
                    |           |
                    v           |
                completed <-----+
```

| Status | Meaning | Transitions to |
|---|---|---|
| `initialized` | Session created, targets collected, codex not yet called | `in_review` (after first `review` call) |
| `in_review` | First review round completed, findings available | `verifying` (when `--session` used), `completed` (when `report` called) |
| `verifying` | A resume/rescan round is in progress or completed | `verifying` (another round), `completed` (when `report` called) |
| `completed` | Report generated, session finalized | (terminal state) |

**Finding statuses:**

| Status | Meaning |
|---|---|
| `open` | Issue identified, not yet addressed |
| `fixed` | Codex verified the fix is correct |
| `dismissed` | User decided not to fix (reason passed to codex via --message) |
| `reopened` | Was fixed/dismissed but rescan found it still present |

Findings are stored directly in `session.json` and updated in-place each round.
No separate `findings.json` or per-round snapshot files. Codex's `--resume` retains
full conversation memory, so xreview does not need to maintain its own history.

---

## 6. Data Formats

### 6.1 Codex Prompt (xreview --> codex)

xreview assembles a prompt from multiple parts and sends it to codex as a single
string via `codex exec`.

#### First Round Prompt

The JSON schema is enforced via `--output-schema` flag, NOT embedded in the prompt.
This keeps the prompt focused on review instructions.

```
<CRITICAL_RULES>
1. PERFORM STATIC ANALYSIS ONLY. Do NOT execute or run the code.
2. Only report issues you can directly observe in the provided code.
   Do NOT speculate about issues in code you cannot see.
3. Every finding MUST reference a specific file and line number.
4. Focus on real bugs and security issues. Do NOT report trivial style preferences.
5. If you find no issues, set verdict to APPROVED with an empty findings array.
6. You are encouraged to read additional files in the repository if needed
   to understand the full context of the code being reviewed.
7. Review comprehensively: security, correctness, readability, maintainability,
   and extensibility. Do NOT limit your review to a single aspect.
8. Suggestions MUST be scoped and actionable within the current change.
   Do NOT suggest large-scale rewrites or architectural overhauls.
   Focus on improvements that can be applied to the code being reviewed.
</CRITICAL_RULES>

You are a senior code reviewer. Analyze the following code changes for bugs,
security vulnerabilities, logic errors, and significant quality issues.

Context from the developer: {context}

===== FILES CHANGED =====

{file_list with brief description of changes}

===== DIFF =====

{unified diff output}

===== END =====
```

**Key design decisions:**

- **Diff + file list instead of full file contents**: Saves tokens, focuses codex
  attention on what changed. Codex can read additional files itself if it needs
  broader context (e.g., to understand a function signature or struct definition).
- **No JSON schema in prompt**: The `--output-schema` flag handles this. Putting
  schema in the prompt wastes tokens and risks conflicting instructions.
- **Encouraging codex to explore**: Rule 6 explicitly tells codex it can read more
  files. This leverages codex's agent capabilities rather than trying to pre-feed
  all context.

**Line number format in file contents:**

```
     1  package auth
     2
     3  import (
     4      "time"
     5      "github.com/golang-jwt/jwt/v5"
     6  )
     7
     8  func ValidateToken(rawToken string) (*Claims, error) {
     9      token, err := jwt.Parse(rawToken, keyFunc)
    10      if err != nil {
    11          return nil, err
    12      }
    13      // BUG: no expiration check
    14      return token.Claims.(*Claims), nil
    15  }
```

Line numbers are included so codex can reference them precisely. The prompt
explicitly notes that line numbers are metadata, not part of the source code.

#### Resume Round Prompt

```
This is a follow-up review. You previously reviewed these files and
identified the findings listed below. The developer has made changes
and provided the following update:

Developer message: "{message}"

===== PREVIOUS FINDINGS =====

F001 [HIGH / security] src/auth.go:42
  JWT token is not checked for expiration.
  Developer update: claims to have fixed this.

F002 [MEDIUM / logic] src/middleware.go:15
  Error returned without context wrapping.
  Developer update: says this is a false positive.

F003 [LOW / error-handling] src/utils.go:88
  Panic on nil pointer not guarded.
  Developer update: no update provided.

===== UPDATED FILES =====

--- src/auth.go ---
{current file contents with line numbers}

--- src/middleware.go ---
{current file contents with line numbers}

--- src/utils.go ---
{current file contents with line numbers}

===== END OF FILES =====

For each previous finding, determine:
1. If claimed fixed: verify the fix is actually correct and complete.
2. If claimed false positive: evaluate whether the dismissal is reasonable.
3. If no update: re-evaluate against the current code.

Also check: did any of the changes introduce NEW issues?

Respond with the same JSON schema. For each finding, set the status:
{
  "findings": [
    {
      "id": "F001",
      "severity": "high | medium | low",
      "category": "security | logic | performance | error-handling",
      "status": "fixed | dismissed | open | reopened",
      "file": "path/to/file.go",
      "line": 42,
      "description": "...",
      "suggestion": "...",
      "code_snippet": "...",
      "verification_note": "Explanation of why this is now fixed/still open/etc."
    }
  ]
}

New findings (not in the previous list) should have status "open" and no "id" field.
```

#### Full Rescan Prompt

Same as the first round prompt but with an additional section listing previous finding
IDs for codex to cross-reference. Codex is smart enough to determine if a previously
identified issue still exists in the current code. xreview does NOT do its own
finding-matching logic — it trusts codex's judgment.

### 6.2 Codex Output Parsing (codex --> xreview)

Because we use `--output-schema`, codex stdout is **clean JSON** that conforms to
our schema. No code fences, no preamble, no extra text. Parsing is straightforward:

```
Codex stdout (clean JSON, guaranteed by --output-schema)
    |
    v
1. json.Unmarshal into FindingsResponse struct
    |
    v
2. Validate required fields are present and non-empty
    |
    v
3. Assign sequential finding IDs (F001, F002, ...)
   For resume rounds, preserve IDs returned by codex
    |
    v
4. Extract session ID from stderr (UUID regex)
    |
    v
5. Write to findings.json and round-NNN.json
   Save raw stdout/stderr to raw/round-NNN-codex-{stdout,stderr}.txt
```

**Parser resilience rules (fallback if --output-schema somehow fails):**

- If stdout is not valid JSON, attempt to strip markdown code fences and re-parse
- If still not parseable, emit `PARSE_FAILURE` error and save raw output
- If a finding is missing optional fields (code_snippet, verification_note), accept with empty values
- If a finding is missing required fields (severity, description), skip it and log a warning

### 6.3 XML Output (xreview --> Claude Code Skill)

All xreview commands output XML on stdout. This is the primary interface between
xreview and the Claude Code skill.

#### Envelope

Every response is wrapped in `<xreview-result>`:

```xml
<xreview-result status="success|error" action="preflight|review|verify|report|version|self-update"
                session="xr-..." round="N">
  ...
</xreview-result>
```

- `status`: "success" or "error"
- `action`: which command produced this output
- `session`: session ID (omitted for preflight/version/self-update)
- `round`: round number (only for review/verify)

#### Review Output (first round)

```xml
<xreview-result status="success" action="review" session="xr-20260310-a1b2c3" round="1">
  <finding id="F001" severity="high" category="security" status="open">
    <location file="src/auth.go" line="42" />
    <description>JWT token is not checked for expiration. An attacker could reuse expired tokens to access the system.</description>
    <suggestion>Add exp claim validation after jwt.Parse: check that claims.ExpiresAt is in the future.</suggestion>
    <code-snippet>token, err := jwt.Parse(rawToken, keyFunc)</code-snippet>
  </finding>

  <finding id="F002" severity="medium" category="logic" status="open">
    <location file="src/middleware.go" line="15" />
    <description>Error returned without context wrapping, making it hard to trace the origin in logs.</description>
    <suggestion>Use fmt.Errorf("middleware auth: %w", err) to preserve the error chain.</suggestion>
    <code-snippet>return err</code-snippet>
  </finding>

  <finding id="F003" severity="low" category="error-handling" status="open">
    <location file="src/utils.go" line="88" />
    <description>Potential nil pointer dereference if config is not initialized.</description>
    <suggestion>Add a nil check before accessing config.Settings.</suggestion>
    <code-snippet>return config.Settings.Timeout</code-snippet>
  </finding>

  <summary total="3" open="3" fixed="0" dismissed="0" />
</xreview-result>
```

#### Verify Output (resume round)

```xml
<xreview-result status="success" action="verify" session="xr-20260310-a1b2c3" round="2">
  <finding id="F001" severity="high" category="security" status="fixed">
    <location file="src/auth.go" line="42" />
    <description>JWT token is not checked for expiration.</description>
    <verification>Fix confirmed. Expiration check correctly added at line 45. The implementation properly rejects tokens where claims.ExpiresAt is in the past.</verification>
  </finding>

  <finding id="F002" severity="medium" category="logic" status="dismissed">
    <location file="src/middleware.go" line="15" />
    <description>Error returned without context wrapping.</description>
    <verification>Dismissal is reasonable. The calling function in handler.go:32 wraps the error with request context before logging.</verification>
  </finding>

  <finding id="F003" severity="low" category="error-handling" status="open">
    <location file="src/utils.go" line="88" />
    <description>Potential nil pointer dereference if config is not initialized.</description>
    <verification>Still present. No changes were made to this code.</verification>
  </finding>

  <finding id="F004" severity="medium" category="logic" status="open">
    <location file="src/auth.go" line="48" />
    <description>New issue: the expiration check uses time.Now() without considering clock skew. Tokens expiring within a few seconds may be incorrectly rejected.</description>
    <suggestion>Add a small grace period (e.g., 30 seconds) when comparing expiration time.</suggestion>
    <code-snippet>if claims.ExpiresAt &lt; time.Now().Unix() {</code-snippet>
  </finding>

  <summary total="4" open="2" fixed="1" dismissed="1" />
</xreview-result>
```

#### Full Rescan Output

When `--full-rescan` is used, xreview compares new findings against the previous round
and annotates each finding with comparison metadata:

```xml
<xreview-result status="success" action="review" session="xr-20260310-a1b2c3" round="3"
                full-rescan="true">
  <finding id="F003" severity="low" category="error-handling" status="open"
           comparison="recurring">
    <location file="src/utils.go" line="88" />
    <description>Potential nil pointer dereference if config is not initialized.</description>
    <suggestion>Add a nil check before accessing config.Settings.</suggestion>
    <code-snippet>return config.Settings.Timeout</code-snippet>
  </finding>

  <finding id="F005" severity="high" category="security" status="open"
           comparison="new">
    <location file="src/auth.go" line="60" />
    <description>Hardcoded secret key used for JWT signing.</description>
    <suggestion>Load the signing key from environment variable or secrets manager.</suggestion>
    <code-snippet>var signingKey = []byte("my-secret-key")</code-snippet>
  </finding>

  <resolved-from-previous>
    <resolved id="F001" note="JWT expiration issue no longer detected in current code." />
    <resolved id="F004" note="Clock skew issue no longer detected in current code." />
  </resolved-from-previous>

  <summary total="2" open="2" fixed="0" dismissed="0"
           previously-resolved="2" new-findings="1" recurring="1" />
</xreview-result>
```

The `comparison` attribute on findings:
- `recurring`: This finding was also present in the previous round
- `new`: This finding was NOT present in the previous round (regression or newly discovered)

The `<resolved-from-previous>` section lists findings from the previous round that
are no longer detected in the rescan.

---

## 7. Codex Integration

### Verified Behavior (from experiments)

The following has been confirmed via real codex CLI testing:

1. **`--output-schema <file>`** enforces structured JSON output. Codex returns
   clean JSON on stdout — no code fences, no preamble, no extra text.
2. **Session ID** is a UUID found in **stderr**. Extract via regex:
   `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`
3. **`additionalProperties: false`** is critical in the JSON schema — without it,
   codex may return unexpected extra fields.
4. **`-c skills.allow_implicit_invocation=false`** disables codex's built-in skills,
   keeping output focused on the review task.

### Spawning Codex

xreview invokes codex as a subprocess using Go's `os/exec` package:

```go
// First round: new session
cmd := exec.CommandContext(ctx, "codex", "exec",
    "-m", model,                             // e.g., "gpt-5.4"
    "--skip-git-repo-check",
    "-c", "skills.allow_implicit_invocation=false",
    "--output-schema", schemaFilePath,       // enforces structured JSON output
    prompt,
)
cmd.Stdout = &stdoutBuf
cmd.Stderr = &stderrBuf
err := cmd.Run()
```

**Key flags:**

| Flag | Purpose | Source |
|---|---|---|
| `-m <model>` | Select the codex model | Configurable via config.json |
| `--skip-git-repo-check` | Allow running outside git repos | Required for nested/non-git contexts |
| `--output-schema <file>` | Enforce JSON schema on stdout | Eliminates need for prompt-based schema |
| `-c skills.allow_implicit_invocation=false` | Disable codex built-in skills | Keeps output focused |

### JSON Schema File

The JSON schema is embedded in the Go binary via `//go:embed` and written to a
temporary file (`os.TempDir()`) before each codex call. The temp file is deleted
after codex returns. No schema files are stored in `.xreview/`.

```go
//go:embed schema/review.json
var reviewSchemaBytes []byte
```

Schema content:

```json
{
  "type": "object",
  "properties": {
    "verdict": {
      "type": "string",
      "enum": ["APPROVED", "REVISE"],
      "description": "Overall review decision"
    },
    "summary": {
      "type": "string",
      "description": "Brief summary of review findings"
    },
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": { "type": "string" },
          "severity": { "type": "string", "enum": ["high", "medium", "low"] },
          "category": { "type": "string", "enum": ["security", "logic", "performance", "error-handling"] },
          "file": { "type": "string" },
          "line": { "type": "integer" },
          "description": { "type": "string" },
          "suggestion": { "type": "string" },
          "code_snippet": { "type": "string" },
          "status": { "type": "string", "enum": ["open", "fixed", "dismissed", "reopened"] },
          "verification_note": { "type": "string" }
        },
        "required": ["id", "severity", "description", "suggestion"],
        "additionalProperties": false
      }
    }
  },
  "required": ["verdict", "summary", "findings"],
  "additionalProperties": false
}
```

The schema is global and shared across all sessions. It is embedded in the binary
at build time and written to a temp file only for the duration of each codex call.

### Session ID Extraction

Session ID is extracted from codex stderr using UUID regex:

```go
func extractCodexSessionID(stderr string) (string, error) {
    re := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
    match := re.FindString(stderr)
    if match == "" {
        return "", fmt.Errorf("no session ID found in codex stderr")
    }
    return match, nil
}
```

The extracted session ID is stored in `session.json` for resume rounds.

### Resuming a Codex Session

```go
// Resume round: pass session ID to codex
cmd := exec.CommandContext(ctx, "codex", "exec",
    "--resume", storedSessionID,
    "-m", model,
    "--skip-git-repo-check",
    "-c", "skills.allow_implicit_invocation=false",
    "--output-schema", schemaFilePath,
    prompt,
)
```

**If resume fails (codex returns error or session expired):**

1. xreview falls back to a fresh codex session
2. The prompt includes full context from session.json (findings, file contents)
3. This is less token-efficient but functionally equivalent
4. The new codex session ID replaces the old one in session.json

### Timeout Handling

```go
ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
defer cancel()

err := cmd.Run()
if ctx.Err() == context.DeadlineExceeded {
    cmd.Process.Kill()
    return XReviewError{Code: "CODEX_TIMEOUT", ...}
}
```

### Stderr Handling

Codex outputs session metadata and progress info on stderr. xreview:

1. Always captures stderr to `raw/round-NNN-codex-stderr.txt`
2. Extracts session ID from stderr (UUID regex)
3. Does NOT parse stderr for findings (only stdout matters)
4. If codex exits with non-zero code, includes stderr content in the error report
5. If `--verbose` flag is set, streams stderr to xreview's own stderr in real-time

---

## 8. Skill Design (Claude Code Integration)

### Skill Location (Day 1)

```
<project>/.claude/skills/xreview/
  SKILL.md           # Main skill file
  reference.md       # XML schema reference for Claude Code
```

### SKILL.md

```yaml
---
name: xreview
description: >
  AI-powered code review using Codex. Triggers after completing a plan or
  milestone to review changed files for bugs, security issues, and logic errors.
  Manages the full review lifecycle: discover, fix, verify, report.
allowed-tools: Bash(xreview *), Bash(go install *), Bash(which *), AskUserQuestion, Read
argument-hint: [files-or-uncommitted]
---

# xreview - Agent-Native Code Review

## Step 0: Ensure xreview is installed

1. Check if xreview exists:
   Run: `which xreview`

2. If NOT found:
   a. Check if Go is available: `which go`
   b. If Go is available:
      - Ask the user: "xreview is not installed. Install it now? (y/n)"
      - If yes: run `go install github.com/davidleitw/xreview@latest`
      - Verify: run `xreview version`
      - If install fails, show the error and stop.
   c. If Go is NOT available:
      - Tell the user: "xreview requires Go to install. Please install Go,
        or download the binary from https://github.com/davidleitw/xreview/releases"
      - Stop.

3. If found, check version:
   Run: `xreview version`
   - Parse the XML output for the `outdated` attribute.
   - If outdated="true":
     Ask user: "xreview {current} is installed but {latest} is available. Update? (y/n)"
     If yes: run `xreview self-update`

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
```

### reference.md

```markdown
# xreview XML Schema Reference

This document describes the XML output format of xreview CLI commands.
Claude Code skill uses this to parse xreview results.

## Envelope

All output is wrapped in:
<xreview-result status="success|error" action="..." session="..." round="N">

## Elements

### <finding>
Attributes: id, severity (high|medium|low), category, status (open|fixed|dismissed|reopened)
Children: <location>, <description>, <suggestion>, <code-snippet>, <verification>

### <location>
Attributes: file (path), line (number)

### <summary>
Attributes: total, open, fixed, dismissed

### <error>
Attributes: code (see error code table)
Content: human-readable error description with suggested action

### <checks> (preflight only)
Children: <check name="..." passed="true|false" detail="..." />

### <version> (version only)
Attributes: current, latest, outdated (true|false), update-command

### <report> (report only)
Attributes: path (file path to generated report)

## Error Codes

| Code | Meaning |
|------|---------|
| CODEX_NOT_FOUND | codex binary not in PATH |
| CODEX_NOT_AUTHENTICATED | codex not logged in |
| CODEX_UNRESPONSIVE | codex did not respond to test prompt |
| CODEX_TIMEOUT | codex exceeded timeout |
| CODEX_ERROR | codex exited with non-zero code |
| PARSE_FAILURE | could not parse codex output |
| SESSION_NOT_FOUND | session ID does not exist |
| NO_TARGETS | no files to review |
| INVALID_FLAGS | invalid flag combination |
| UPDATE_FAILED | self-update failed |
```

### Skill Trigger Strategy

The skill is designed to be invoked in two ways:

1. **Manual:** User types `/xreview` in Claude Code
2. **Suggested by Claude Code:** After completing a plan step, Claude Code sees the
   skill description and suggests running it. The description keyword "after completing
   a plan or milestone" helps Claude Code recognize when to suggest it.

The skill does NOT auto-trigger. It always asks the user "Code review? (y/n)" before
doing anything.

---

## 9. Error Handling

### Design Philosophy

Every error message from xreview is written for an AI agent (Claude Code) to read
and act upon. Messages must be:

1. **Descriptive:** Clearly state what went wrong
2. **Actionable:** Include a specific suggested fix or next step
3. **Context-rich:** Include enough detail for Claude Code to explain to the user

### Error Output Format

All errors follow the same XML envelope:

```xml
<xreview-result status="error" action="{command}">
  <error code="{ERROR_CODE}">
    {Descriptive message with suggested action}
  </error>
</xreview-result>
```

### Complete Error Code Reference

| Code | Command | Condition | Message |
|---|---|---|---|
| `CODEX_NOT_FOUND` | preflight | `exec.LookPath("codex")` fails | "codex CLI is not found in PATH. Please install it. If using npm: npm install -g @openai/codex" |
| `CODEX_NOT_AUTHENTICATED` | preflight | codex auth check fails | "codex is installed but not logged in. Please ask the user to run: codex login" |
| `CODEX_UNRESPONSIVE` | preflight | codex test prompt times out or errors | "codex is installed and authenticated but not responding. This could be a network issue or service outage. Try again in a moment." |
| `CODEX_TIMEOUT` | review | codex exceeds --timeout | "codex did not respond within {N} seconds. The review may be too large, or there may be a network issue. Suggestions: retry with --timeout {N*2}, or reduce the number of files." |
| `CODEX_ERROR` | review | codex exits non-zero | "codex exited with error code {N}. stderr output saved to {path}. This usually means codex encountered an internal error. Try running the review again." |
| `PARSE_FAILURE` | review | cannot extract JSON from codex output | "Could not parse codex output as the expected JSON format. The raw output has been saved to {path} for debugging. This may be a prompt issue — try running the review again." |
| `SESSION_NOT_FOUND` | review, report | --session ID not in .xreview/sessions/ | "Session '{id}' not found in .xreview/sessions/. Available sessions: {list}. Check the session ID and try again." |
| `NO_TARGETS` | review | no files specified and --git-uncommitted found no changes | "No files to review. Either specify files with --files, or make some changes and use --git-uncommitted." |
| `INVALID_FLAGS` | any | conflicting or missing required flags | "Invalid flag combination: {specific issue}. {How to fix it}." |
| `FILE_NOT_FOUND` | review | a file in --files does not exist | "File not found: {path}. Please check the file path and try again." |
| `NOT_GIT_REPO` | review | --git-uncommitted used outside a git repo | "Not inside a git repository. --git-uncommitted requires a git repo. Use --files to specify files explicitly instead." |
| `UPDATE_FAILED` | self-update | go install or download fails | "Failed to update xreview: {underlying error}. You can try manually: go install github.com/davidleitw/xreview@latest" |
| `VERSION_CHECK_FAILED` | version | cannot reach GitHub API | "Could not check for updates (GitHub API unreachable). Current version: {version}. This is not critical — you can still use xreview normally." |

### Exit Codes

| Exit code | Meaning |
|---|---|
| 0 | Success (status="success" in XML) |
| 1 | Error (status="error" in XML, error details in output) |
| 2 | Invalid usage (bad flags, missing arguments — prints usage help to stderr) |

The skill should primarily rely on the XML output, not exit codes. Exit codes are
a backup for cases where XML output might not be generated (e.g., binary crash).

### Stderr vs Stdout

| Stream | Content | Consumer |
|---|---|---|
| stdout | XML output (always structured) | Claude Code skill |
| stderr | Debug logs (only with --verbose), usage errors | Human developer (debugging) |

xreview NEVER mixes unstructured text into stdout. If something goes wrong before
XML can be generated (e.g., flag parsing error), the error goes to stderr and the
process exits with code 2.

---

## 10. Testing Strategy

### Test Pyramid

```
                  /  E2E Tests  \          Real codex, prepared test repo
                 /   (CI-gated)  \         Env: XREVIEW_E2E=1
                /-------------------\
               / Integration Tests   \     Mock codex binary, full lifecycle
              /   (always in CI)      \
             /-------------------------\
            /      Unit Tests           \  Pure Go, no external dependencies
           /    (always in CI)           \
          /-------------------------------\
```

### Unit Tests

Pure Go tests with no external dependencies (no codex, no git, no filesystem for
most tests).

| Package | What to test | Example |
|---|---|---|
| `internal/collector` | File reading, uncommitted change detection | Given file paths, returns correct content with line numbers |
| `internal/prompt` | Prompt assembly for first round and resume | Given targets + context + previous findings, produces correct prompt string |
| `internal/parser` | JSON extraction from codex output | Given raw output with/without code fences, extracts findings correctly |
| `internal/parser` | Handling of malformed output | Missing fields, extra text, invalid JSON, empty response |
| `internal/formatter` | XML generation | Given findings, produces valid XML matching expected output |
| `internal/formatter` | Error XML generation | Each error code produces correct XML error output |
| `internal/session` | Session CRUD | Create, read, update session.json; finding state transitions |
| `internal/session` | Round management | Write round-NNN.json, track history correctly |
| `internal/session` | Finding comparison (for full-rescan) | Given old and new findings, correctly identifies recurring/new/resolved |
| `internal/version` | Version parsing and comparison | Semver comparison, outdated detection |
| `cmd/` | Flag validation | Conflicting flags, missing required flags produce correct errors |

**Fixtures directory:**

```
test/fixtures/
  codex-output/
    clean-json.txt              # Perfect JSON output
    json-in-code-fence.txt      # JSON wrapped in ```json ... ```
    json-with-preamble.txt      # Explanatory text + JSON
    malformed-json.txt          # Invalid JSON
    empty-findings.txt          # {"findings": []}
    missing-fields.txt          # Findings with missing required fields
  sessions/
    sample-session/             # Complete session directory for testing
      session.json
      findings.json
      rounds/
        round-001.json
```

### Integration Tests (Mock Codex)

Replace the real codex binary with a mock shell script that returns predictable
responses based on input patterns.

**Mock codex binary (`test/mock-codex/codex`):**

```bash
#!/bin/bash
# Mock codex binary for integration testing.
# Routes based on command and prompt content.

# Handle auth status check
if [[ "$1" == "auth" && "$2" == "status" ]]; then
    echo '{"status": "authenticated", "user": "test@example.com"}'
    exit 0
fi

# Handle exec command
if [[ "$1" == "exec" ]]; then
    # Extract the prompt (last argument)
    prompt="${@: -1}"

    # Route based on prompt content
    if echo "$prompt" | grep -q "respond with OK"; then
        # Preflight responsiveness check
        echo "OK"
        exit 0
    fi

    if echo "$prompt" | grep -q "follow-up review"; then
        # Resume/verify response
        cat "$(dirname "$0")/../fixtures/codex-output/verify-response.json"
        exit 0
    fi

    # Default: first round review response
    cat "$(dirname "$0")/../fixtures/codex-output/review-response.json"
    exit 0
fi

echo "Unknown command: $@" >&2
exit 1
```

**Integration test setup:**

```go
func TestMain(m *testing.M) {
    // Prepend mock codex directory to PATH
    mockDir := filepath.Join("test", "mock-codex")
    os.Setenv("PATH", mockDir + ":" + os.Getenv("PATH"))

    // Use a temp directory for .xreview/
    tmpDir, _ := os.MkdirTemp("", "xreview-test-*")
    os.Setenv("XREVIEW_WORKDIR", tmpDir)
    defer os.RemoveAll(tmpDir)

    os.Exit(m.Run())
}
```

**Integration test scenarios:**

| Scenario | Steps | Validates |
|---|---|---|
| Happy path: full lifecycle | preflight -> review -> verify -> report | All commands work together, session state persists correctly |
| Resume with codex session | review -> get session ID -> review --session -> check codex_resumed=true | Codex session ID is stored and passed correctly |
| Full rescan | review -> review --session --full-rescan | New codex session created, findings compared against previous |
| Error: codex not found | Remove mock from PATH, run preflight | CODEX_NOT_FOUND error with correct message |
| Error: codex timeout | Mock codex with `sleep 999`, run review --timeout 1 | CODEX_TIMEOUT error, process killed |
| Error: parse failure | Mock codex returns invalid JSON, run review | PARSE_FAILURE error, raw output saved |
| Error: session not found | Run review --session nonexistent-id | SESSION_NOT_FOUND error with available sessions listed |
| Error: no targets | Run review --git-uncommitted in empty repo | NO_TARGETS error |
| Error: file not found | Run review --files nonexistent.go | FILE_NOT_FOUND error |
| Multiple rounds | review -> verify -> verify -> verify -> report | Round numbers increment, history tracked correctly |
| Finding state transitions | open -> fixed, open -> dismissed, dismissed -> reopened | Each transition recorded in history |

### E2E Tests (Real Codex)

These tests use the real codex CLI and are gated behind an environment variable.

**Gate:** `XREVIEW_E2E=1`

**Setup:** A prepared git repository with known bugs:

```
test/e2e/test-repo/
  main.go          # Contains: SQL injection, unchecked error, hardcoded secret
  auth.go          # Contains: missing JWT expiration check
  README.md        # Clean file (should not be flagged)
```

**E2E test scenarios:**

| Scenario | Expected |
|---|---|
| Review finds known bugs | Findings include SQL injection, unchecked error, hardcoded secret |
| Fix bugs, verify | After fixing, codex confirms fixes are correct |
| False positive handling | Dismiss a finding, verify codex accepts the dismissal rationale |
| Clean code review | Review README.md, expect zero findings |

**E2E tests are NOT run on every CI push.** They are:
- Run manually during development
- Run in a dedicated CI job on release branches
- Require a valid codex authentication

---

## 11. Project Structure (Go)

```
xreview/
  cmd/
    xreview/
      main.go                    # Entry point, cobra root command
      cmd_preflight.go           # preflight subcommand
      cmd_review.go              # review subcommand
      cmd_report.go              # report subcommand
      cmd_clean.go               # clean subcommand
      cmd_version.go             # version subcommand
      cmd_selfupdate.go          # self-update subcommand
  internal/
    collector/
      collector.go               # File content collection (files + directory expansion)
      collector_test.go
      git.go                     # Git uncommitted change detection
      git_test.go
    prompt/
      builder.go                 # Prompt assembly (first round + resume)
      builder_test.go
      templates.go               # Prompt template strings
    parser/
      parser.go                  # Codex output JSON parsing
      parser_test.go
      extract.go                 # JSON extraction from raw text
      extract_test.go
    formatter/
      xml.go                     # XML output generation
      xml_test.go
      error.go                   # Error XML generation
      error_test.go
    session/
      manager.go                 # Session CRUD (single session.json per session)
      manager_test.go
      types.go                   # Session, Finding structs
    codex/
      runner.go                  # Codex process spawning and management
      runner_test.go
      resume.go                  # Session resume logic + session ID extraction
      resume_test.go
    schema/
      review.json                # Embedded via //go:embed — codex output schema
      schema.go                  # Writes embedded schema to temp file for codex
      schema_test.go
    version/
      version.go                 # Version check and self-update
      version_test.go
    config/
      config.go                  # Project-level config (.xreview/config.json)
      config_test.go
    reviewer/
      reviewer.go                # Reviewer interface (Day 1: SingleReviewer)
      single.go                  # SingleReviewer — single codex call
      single_test.go
  test/
    fixtures/
      codex-output/              # Sample codex outputs for parser tests
      sessions/                  # Sample session.json files
    mock-codex/
      codex                      # Mock codex binary (shell script)
    e2e/
      e2e_test.go                # E2E tests (gated behind XREVIEW_E2E=1)
      test-repo/                 # Prepared repo with known bugs
  docs/
    specs/
      2026-03-10-xreview-design.md   # This document
  .claude/
    skills/
      xreview/
        SKILL.md                 # Claude Code skill
        reference.md             # XML schema reference
  go.mod
  go.sum
  Makefile
  README.md
```

### Key Dependencies

| Dependency | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI framework (subcommands, flags, help text) |
| `encoding/json` | JSON parsing (stdlib) |
| `encoding/xml` | XML generation (stdlib) |
| `os/exec` | Codex process management (stdlib) |
| `text/template` | Prompt template rendering (stdlib) |

Minimal external dependencies. Most functionality uses Go stdlib.

### Build & Development

```makefile
# Makefile
.PHONY: build test test-integration test-e2e lint

build:
	go build -o bin/xreview ./cmd/xreview

test:
	go test ./internal/... ./cmd/...

test-integration:
	go test ./test/... -tags=integration

test-e2e:
	XREVIEW_E2E=1 go test ./test/e2e/... -tags=e2e -timeout 300s

lint:
	golangci-lint run ./...

install:
	go install ./cmd/xreview
```

---

## 12. Future: Multi-Agent Review (Option B)

Day 1 uses a single codex call with a general review prompt. The architecture is
designed to evolve into multi-agent parallel review without changing the CLI interface.

### Concept

Instead of one codex call with a general prompt, spawn N codex processes in parallel,
each with a specialized reviewer persona:

```
xreview review --files src/auth.go
    |
    +---> codex (security reviewer)      --> security findings
    |
    +---> codex (logic reviewer)         --> logic findings
    |
    +---> codex (performance reviewer)   --> performance findings
    |
    v
    xreview aggregator
    |
    +---> merge duplicates
    +---> consensus scoring
    +---> unified findings list
    |
    v
    XML output (same format as Day 1)
```

### Consensus Scoring (inspired by Calimero ai-code-reviewer)

When multiple reviewers independently flag the same issue, confidence increases:

```
confidence = agreement_count / total_reviewers
final_priority = severity_weight * confidence
```

- Finding flagged by 3/3 reviewers: high confidence, high priority
- Finding flagged by 1/3 reviewers: lower confidence, may be false positive

This naturally filters out false positives — if only the security reviewer flags
a style issue that the logic reviewer doesn't care about, it gets lower priority.

### Reviewer Profiles

Configurable via `.xreview/reviewers/` directory:

```yaml
# .xreview/reviewers/security.yaml
name: security
persona: >
  You are a senior security engineer. Focus exclusively on:
  authentication, authorization, injection vulnerabilities,
  secrets exposure, input validation, and cryptographic misuse.
  Do NOT report general code quality issues.
severity_boost:
  - security    # Boost severity for security-category findings
```

```yaml
# .xreview/reviewers/logic.yaml
name: logic
persona: >
  You are a senior software engineer focused on correctness.
  Look for: logic errors, off-by-one bugs, nil/null pointer issues,
  race conditions, incorrect error handling, and edge cases.
  Do NOT report style or performance issues.
severity_boost:
  - logic
  - error-handling
```

### Aggregation Algorithm

```
1. Collect findings from all reviewers
2. Group findings by (file, line_range):
   - Exact line match: same finding
   - Within 5 lines of each other with similar description: likely same finding
3. For each group:
   - Merge descriptions (take the most detailed one)
   - Set confidence = group_size / total_reviewers
   - Set severity = max(individual severities) * confidence_boost
4. Sort by final_priority descending
5. Assign finding IDs (F001, F002, ...)
```

### What Changes from Day 1

| Component | Day 1 (Single Agent) | Future (Multi-Agent) |
|---|---|---|
| codex calls per round | 1 | N (parallel) |
| Prompt | General review | Specialized per reviewer persona |
| Post-processing | Direct JSON parse | Parse N outputs + merge + deduplicate + score |
| Session state | Single `codex_session_id` | Map of `reviewer_name -> codex_session_id` |
| Resume | Resume single codex session | Resume N codex sessions in parallel |
| Config | None | `.xreview/reviewers/*.yaml` |
| CLI changes | None | Optional: `--reviewers security,logic` to select which reviewers to use |

### Migration Path (Go Interface)

```go
// Reviewer abstracts single vs. multi-agent review.
type Reviewer interface {
    Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error)
    Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error)
}

// Day 1: single codex call
type SingleReviewer struct {
    codex  *codex.Runner
    prompt *prompt.Builder
}

// Future: parallel codex calls with aggregation
type MultiReviewer struct {
    reviewers  []SingleReviewer   // Each with a different persona
    aggregator *Aggregator        // Merge + deduplicate + score
}

// The CLI commands use the Reviewer interface, not concrete types.
// Switching from Single to Multi is a config change, not a code change.
```

No CLI interface changes needed. `xreview review` output format stays the same.
The multi-agent behavior is purely internal. Users (and the Claude Code skill)
see the same XML output regardless of how many codex processes ran behind the scenes.

---

## 13. Open Questions

### Must resolve before implementation

1. **Codex exec exact CLI syntax:** Need to verify the exact flags for `codex exec`
   (model selection, session resume, output capture). Run experiments with the
   script in `/tmp/c.py` as a reference.

2. **Codex session resume mechanism:** How exactly does codex expose session IDs?
   Is it in stdout, stderr, or a separate metadata channel? This determines how
   xreview captures and stores the session ID.

3. **Codex structured output reliability:** How reliably does codex return valid
   JSON when prompted to? Do we need retry logic (re-prompt once if parse fails)?

### Nice to resolve before implementation

4. **Token budget management:** Should xreview enforce a maximum total file size
   before sending to codex? Options:
   - Hard limit (e.g., 100KB total) with error message
   - Automatic truncation of large files
   - Let codex handle it (simplest, but may produce poor results)

5. **Project conventions injection:** Should xreview auto-detect and include project
   conventions (`.cursor/rules`, `CLAUDE.md`, `.editorconfig`, linter configs) in the
   codex prompt to improve review quality?

### Future considerations

6. **Git diff mode:** Support `--base <ref>` for branch-based diffs (e.g.,
   `--base main` to review all changes since branching from main).

7. **Rate limiting for multi-agent:** If using Option B, should xreview limit
   concurrent codex processes? (Probably: configurable max parallelism in config.json)

8. **Caching:** Should xreview cache codex responses for identical file contents?
   (Probably not for Day 1 — review results should be fresh.)

9. **CI integration:** Should xreview support a non-interactive mode for CI pipelines?
   (e.g., `xreview review --ci --files ... --fail-on high` that exits non-zero if
   high-severity findings are found.)

---

## 14. Appendix: Claude Code Skill & Plugin Reference

This appendix is a practical reference for building the xreview skill and plugin.
No conceptual explanations — only exact syntax, valid values, and working examples.

Official docs:
- Skills: https://code.claude.com/docs/en/skills.md
- Plugins: https://code.claude.com/docs/en/plugins.md
- Plugins Reference: https://code.claude.com/docs/en/plugins-reference.md
- Hooks: https://code.claude.com/docs/en/hooks.md
- Permissions: https://code.claude.com/docs/en/permissions.md

### 14.1 Creating a Skill

A skill is a directory containing a `SKILL.md` file with YAML frontmatter.

**Skill locations (higher priority wins):**

| Priority | Location | Scope |
|---|---|---|
| 1 (highest) | Enterprise managed settings | All org users |
| 2 | `~/.claude/skills/<name>/SKILL.md` | All your projects |
| 3 | `.claude/skills/<name>/SKILL.md` | Current project only |
| 4 (lowest) | `<plugin>/skills/<name>/SKILL.md` | When plugin enabled |

Plugin skills are namespaced: `/plugin-name:skill-name`.
Non-plugin skills use direct names: `/skill-name`.

**Monorepo auto-discovery:** Working in `packages/frontend/` automatically discovers
skills from both `.claude/skills/` (root) and `packages/frontend/.claude/skills/`
(nested).

### 14.2 SKILL.md Frontmatter — All Fields

```yaml
---
name: my-skill                      # Slash command name. Lowercase, numbers, hyphens. Max 64 chars.
                                    # If omitted, uses directory name.
description: >                      # When to use this skill. Claude reads this to decide auto-invocation.
  Describe what this skill does     # If omitted, uses first paragraph of markdown content.
  and when to use it.
argument-hint: "[file] [format]"    # Shown during autocomplete. Purely cosmetic.
allowed-tools: Bash(xreview *), Read, Grep  # Tools allowed without permission prompt.
disable-model-invocation: false     # true = only human can invoke (/skill-name). Claude cannot auto-load.
user-invocable: true                # false = only Claude can invoke. Hidden from / menu.
model: opus                         # Override model when skill is active.
context: fork                       # "fork" = run in isolated subagent context.
agent: Explore                      # Subagent type when context=fork. Options: Explore, Plan, general-purpose, or custom.
---
```

**Invocation control matrix:**

| Setting | Human can invoke | Claude can invoke | In context budget |
|---|---|---|---|
| Default | Yes | Yes | Description loaded |
| `disable-model-invocation: true` | Yes | No | Description NOT loaded |
| `user-invocable: false` | No | Yes | Description loaded |

### 14.3 String Substitutions in SKILL.md

| Variable | Expands to | Example |
|---|---|---|
| `$ARGUMENTS` | All arguments as a single string | `/review src/auth.go` -> `"src/auth.go"` |
| `$ARGUMENTS[0]` or `$0` | First argument | `/review src/auth.go thorough` -> `"src/auth.go"` |
| `$ARGUMENTS[1]` or `$1` | Second argument | -> `"thorough"` |
| `${CLAUDE_SESSION_ID}` | Current Claude Code session UUID | `"abc-123-def"` |
| `${CLAUDE_SKILL_DIR}` | Absolute path to the skill's directory | `"/home/user/.claude/skills/xreview"` |

If `$ARGUMENTS` is not present anywhere in the content, arguments are appended
as `ARGUMENTS: <value>` at the end.

### 14.4 Dynamic Context Injection

The `` !`command` `` syntax runs a shell command **before** the skill content is
sent to Claude. The output replaces the placeholder.

```yaml
---
name: pr-review
---

Current diff:
!`git diff HEAD~1`

Recent commits:
!`git log --oneline -5`

Review the above changes.
```

Claude receives the actual diff and log output, not the commands.

### 14.5 allowed-tools Syntax

Comma or space-separated. Case-sensitive tool names.

**Tool names:**
`Read`, `Edit`, `Write`, `Bash`, `Grep`, `Glob`, `WebFetch`, `WebSearch`,
`Agent`, `Skill`, `AskUserQuestion`, `mcp__<server>__<tool>`

**Bash patterns (glob-style):**

```yaml
allowed-tools: Bash(xreview *)      # xreview with any args
allowed-tools: Bash(npm run *)      # npm run with any args
allowed-tools: Bash(git commit *)   # git commit with any args
allowed-tools: Bash(*)              # all bash commands
allowed-tools: Bash                 # also all bash commands
```

Word boundary: `Bash(npm *)` matches `npm test` but NOT `npms`.
No word boundary: `Bash(npm*)` matches both `npm test` and `npms`.

**Multiple tools:**

```yaml
allowed-tools: Bash(xreview *), Bash(go install *), Bash(which *), Read, AskUserQuestion
```

### 14.6 Plugin Structure

```
my-plugin/
  .claude-plugin/              # REQUIRED directory
    plugin.json                # REQUIRED manifest
  skills/                      # Skill directories
    my-skill/
      SKILL.md
  hooks/
    hooks.json                 # Hook definitions
  scripts/                     # Supporting scripts
    install.sh
```

**CRITICAL:** `.claude-plugin/` contains ONLY `plugin.json`. Everything else goes
at the plugin root.

### 14.7 plugin.json — All Fields

```json
{
  "name": "my-plugin",                          // REQUIRED. Kebab-case. Becomes namespace prefix.
  "version": "1.0.0",                           // Semver.
  "description": "What this plugin does",       // Shown in plugin manager.
  "author": {
    "name": "Author Name",
    "email": "author@example.com",
    "url": "https://github.com/author"
  },
  "homepage": "https://docs.example.com",
  "repository": "https://github.com/author/plugin",
  "license": "MIT",
  "keywords": ["keyword1", "keyword2"],

  "skills": "./skills/",                        // Path to skills directory (relative, starts with ./)
  "hooks": "./hooks/hooks.json",                // Path to hooks config
  "commands": ["./commands/deploy.md"],          // Additional command files
  "agents": "./agents/",                         // Agent definition directory
  "mcpServers": "./.mcp.json",                   // MCP server definitions
  "lspServers": "./.lsp.json",                   // LSP server definitions
  "outputStyles": "./styles/"                    // Output style files
}
```

All component paths are relative and must start with `./`.

**Environment variable in plugins:**
`${CLAUDE_PLUGIN_ROOT}` — absolute path to the plugin directory at runtime.
Use in hooks, scripts, and MCP server configs.

### 14.8 Hooks

#### Hook File Format

```json
{
  "hooks": {
    "<EventName>": [
      {
        "matcher": "<regex>",
        "hooks": [
          {
            "type": "command",
            "command": "shell command here"
          }
        ]
      }
    ]
  }
}
```

#### Hook Events

| Event | Fires when | Matcher filters on |
|---|---|---|
| `SessionStart` | Session begins/resumes | `startup`, `resume`, `clear`, `compact` |
| `UserPromptSubmit` | User sends message | (no matcher) |
| `PreToolUse` | Before tool executes | Tool name |
| `PostToolUse` | After tool succeeds | Tool name |
| `PostToolUseFailure` | After tool fails | Tool name |
| `PermissionRequest` | Permission dialog shown | Tool name |
| `Notification` | Notification sent | `permission_prompt`, `idle_prompt`, `auth_success` |
| `SubagentStart` | Subagent spawned | Agent type: `Explore`, `Plan`, custom |
| `SubagentStop` | Subagent finishes | Agent type |
| `Stop` | Claude finishes responding | `manual`, `auto` |
| `TaskCompleted` | Task marked complete | (no matcher) |
| `PreCompact` | Before context compaction | `manual`, `auto` |
| `SessionEnd` | Session terminates | `clear`, `logout`, `other` |

#### Hook Types

| Type | Description |
|---|---|
| `command` | Shell command. Receives JSON on stdin. |
| `http` | POST to URL with JSON body. |
| `prompt` | Single LLM call. |
| `agent` | Multi-turn agent with tools. |

#### Hook Input (stdin for command hooks)

```json
{
  "session_id": "abc123",
  "cwd": "/path/to/project",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {
    "command": "npm test"
  }
}
```

#### Hook Exit Codes

| Exit code | Effect |
|---|---|
| 0 | Action proceeds. Stdout added to Claude's context. |
| 2 | Action BLOCKED. Stderr shown to Claude as feedback. |
| Other | Action proceeds. Stderr logged but not shown. |

### 14.9 Permissions in settings.json

```json
{
  "permissions": {
    "allow": [
      "Bash(xreview *)",
      "Bash(go install github.com/davidleitw/xreview*)",
      "Read",
      "Skill(xreview:review)"
    ],
    "deny": [
      "Bash(rm -rf *)",
      "Edit(.env)"
    ]
  }
}
```

**Precedence:** deny > ask > allow. First match wins.

**Path patterns for Read/Edit (gitignore spec):**
- `/path` — relative to project root
- `~/path` — relative to home
- `//path` — absolute path
- `**` — recursive matching

### 14.10 Plugin Installation & Testing

```bash
# Local development (no install needed)
claude --plugin-dir ./xreview-plugin

# Install commands
claude plugin install <name>                  # From marketplace
claude plugin install <name>@<marketplace>    # From specific marketplace
claude plugin install <name> --scope user     # Personal (default)
claude plugin install <name> --scope project  # Shared via git
claude plugin install <name> --scope local    # Gitignored

# Management
claude plugin enable <name>
claude plugin disable <name>
claude plugin update <name>
claude plugin uninstall <name>
```

### 14.11 Context Budget

Skill descriptions consume context window space:
- **Budget:** 2% of context window (fallback: 16,000 chars if 2% is too small)
- **Override:** `SLASH_COMMAND_TOOL_CHAR_BUDGET` environment variable
- If too many skills, some descriptions get excluded. Run `/context` to check.

### 14.12 Practical Example: xreview as Day 1 Project Skill

```
<project>/
  .claude/
    skills/
      xreview/
        SKILL.md          # /xreview
        reference.md      # XML schema docs
```

```yaml
# .claude/skills/xreview/SKILL.md
---
name: xreview
description: >
  AI-powered code review using Codex. Use after completing a plan or
  milestone to review changed files for bugs, security issues, and logic errors.
allowed-tools: Bash(xreview *), Bash(go install *), Bash(which *), Read, AskUserQuestion
argument-hint: [files-or-uncommitted]
---

# xreview

## Step 0: Ensure xreview is installed
...
```

Invocation: `/xreview` or `/xreview src/auth.go,src/middleware.go`

### 14.13 Practical Example: xreview as Plugin

```
xreview-plugin/
  .claude-plugin/
    plugin.json
  skills/
    review/
      SKILL.md            # /xreview:review
      reference.md
  hooks/
    hooks.json
  scripts/
    install-binary.sh
```

```json
// .claude-plugin/plugin.json
{
  "name": "xreview",
  "version": "0.1.0",
  "description": "Agent-native code review engine powered by Codex",
  "skills": "./skills/",
  "hooks": "./hooks/hooks.json"
}
```

```yaml
# skills/review/SKILL.md
---
name: review
description: >
  AI-powered code review using Codex. Use after completing a plan or
  milestone to review changed files.
allowed-tools: Bash(xreview *), Bash(go install *), Bash(which *), Read, AskUserQuestion
---

## Step 0: Ensure xreview is installed

1. Run: `which xreview`
2. If not found: `${CLAUDE_PLUGIN_ROOT}/scripts/install-binary.sh`
...
```

Invocation: `/xreview:review` or `/xreview:review src/auth.go`

Testing: `claude --plugin-dir ./xreview-plugin`
