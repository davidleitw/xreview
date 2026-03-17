# xreview

Agent-native code review engine for Claude Code, powered by Codex.

xreview delegates code review to Codex (a separate AI model) so Claude Code gets an independent second opinion. It orchestrates a three-party review loop: **Codex reviews, Claude Code fixes, you decide.**

**[中文版 README](docs/README.zh-TW.md)**

## How It Works

When you ask Claude Code to review your code, the xreview skill takes over:

1. **Codex reviews** your code and reports findings (bugs, security issues, logic errors)
2. **Claude Code verifies** each finding independently — reads the actual source code, confirms or challenges false positives by discussing with Codex
3. **Claude Code presents** a Fix Plan with only verified findings — trigger, impact, cascade, and fix options
4. **You decide** — approve all recommended fixes, pick by severity, or adjust per finding
5. **Claude Code fixes** strictly per your approved plan
6. **Codex verifies** the fixes in a follow-up round, may find new issues or reopen dismissed ones
7. **Repeat** until all parties agree (or 5 rounds max)
8. **Report generated** — a human-readable markdown report summarizing findings, decisions, and fixes

This isn't Claude Code reviewing its own work. It's a genuinely independent review from a different model, with Claude Code acting as a verification layer that filters out false positives before presenting to you.

## Installation

### Claude Code

Register the marketplace and install:

```bash
/plugin marketplace add davidleitw/xreview
/plugin install xreview@xreview-marketplace
```

### Prerequisites

- [Codex CLI](https://github.com/openai/codex) installed and authenticated (`npm install -g @openai/codex`)
- OpenAI API key configured for Codex

## Usage

Just ask Claude Code to review your code:

```
Review my code for bugs and security issues
```

Or be specific about which files:

```
Review store/db.go and handler/exec.go for security vulnerabilities
```

The xreview skill triggers automatically. You can also invoke it directly:

```
/xreview
```

### What It Catches

| Category | Examples |
|----------|---------|
| **Security** | SQL injection, command injection, hardcoded secrets, missing auth |
| **Logic** | Nil pointer dereference, race conditions, off-by-one errors |
| **Error Handling** | Ignored errors, resource leaks, unclosed connections |
| **Performance** | N+1 queries, unnecessary allocations |

### The Three-Party Loop

Each finding goes through a structured analysis:

```
F-001: SQL Injection (security/high)
  store/db.go:34 — FindUser()

Trigger: user sends malicious string via /user?name=' OR '1'='1
Root cause: fmt.Sprintf concatenates user input directly into SQL query
Impact: attacker can read, modify, or delete any data in the database

-> Fix: changed to parameterized query db.Query("...WHERE name = ?", name)
```

- **All findings presented at once** — you see the full picture before any code changes
- **Multiple fix options per finding** — Claude Code lists alternatives with effort levels; you pick
- **Every finding includes "Don't fix"** — you always have the final say

After all findings are addressed, Codex verifies the fixes. If it disagrees with a dismissal or finds an incomplete fix, the loop continues.

## Auto-Update

xreview keeps itself up to date automatically. During preflight (the first step of every review), it checks GitHub Releases for a newer version. The check is cached locally for 24 hours to avoid slowing things down.

When a new version is available, the skill runs `xreview self-update` before proceeding. The update downloads a pre-built binary matching your OS and architecture — no Go toolchain required. If the update fails for any reason, xreview continues with the current version.

You can also update manually:

```bash
xreview self-update
```

## CLI Reference

xreview ships as a standalone Go binary that Claude Code calls under the hood:

| Command | Purpose |
|---------|---------|
| `xreview preflight` | Check environment (codex installed, API key, version, updates) |
| `xreview review --files <paths>` | Run initial review |
| `xreview review --session <id> --message "..."` | Resume for verification round |
| `xreview report --session <id>` | Generate final report |
| `xreview clean --session <id>` | Clean up session data |
| `xreview self-update` | Update to the latest version from GitHub Releases |
| `xreview version` | Show version |

## Development

```bash
git clone https://github.com/davidleitw/xreview.git
cd xreview
go build -o xreview ./cmd/xreview/
```

To load the plugin locally in Claude Code (without installing from marketplace):

```bash
claude --plugin-dir .
```

This loads `skills/` from the repo root via `.claude-plugin/plugin.json`. Use `/reload-plugins` inside the session to hot-reload after editing skill files.

## Architecture

```
Claude Code (host)          xreview (CLI)           Codex (reviewer)
     |                          |                        |
     |-- /xreview skill ------->|                        |
     |                          |-- codex exec --------->|
     |                          |   (Codex reads code    |
     |                          |    via git diff/files)  |
     |                          |<-- findings (JSON) ----|
     |                          |  [snapshot file         |
     |                          |   checksums]            |
     |<-- findings (XML) ------|                        |
     |                          |                        |
     |  [verify each finding]   |                        |
     |  [challenge suspects] -->|-- codex resume ------->|
     |                          |<-- re-evaluate --------|
     |                          |                        |
     |  [present Fix Plan]      |                        |
     |  [user approves]         |                        |
     |  [fix code]              |                        |
     |                          |                        |
     |-- resume --------------->|  [detect changed files |
     |                          |   via checksum diff]    |
     |                          |-- codex resume ------->|
     |                          |   (prompt includes      |
     |                          |    changed file list)   |
     |                          |<-- verify (JSON) ------|
     |<-- verify (XML) --------|                        |
     |                          |                        |
     |  [write-report skill]    |                        |
     |-- report --------------->|                        |
     |<-- session data ---------|                        |
     |  [generate markdown]     |                        |
```

- xreview outputs XML on stdout for Claude Code skill consumption
- Codex fetches code itself (runs `git diff` or reads files in read-only mode)
- Claude Code independently verifies each finding before presenting to user
- Internal state stored as JSON in `.xreview/sessions/`
- Multi-round: codex session resume via `--resume <session-id>`
- File snapshot (SHA-256 checksums) tracks changes between rounds — xreview detects which files changed and tells Codex to re-read them, ensuring reviews always evaluate the latest code
- Human-readable markdown report generated by write-report skill

## Future Work

See [Roadmap & Design](docs/specs/2026-03-17-roadmap-next-generation-review.md) for the full plan. Key directions:

- **Context engineering** — structured context files and focused review angles, letting Claude Code prepare architectural context (symbol cross-references, call chains, data structure shapes) before Codex reviews. Solves the "semantic gap" where code works correctly but communicates intent poorly.
- **Multi-angle review** — dispatch multiple parallel Codex reviews, each focused on a different concern (semantic consistency, lifecycle naming, bugs/security), then merge and deduplicate findings. Claude Code decides when multi-angle is warranted based on code complexity.
- **Design plan review** — review implementation plans and design docs before execution, checking for feasibility issues, missing edge cases, and architectural conflicts with existing code.
- **Multi-model review** — run the same code through independent reviewers (Codex, Gemini, local models) and cross-validate findings. Different models have different blind spots; cross-model consensus yields higher-confidence findings.
- **Language-aware review context** — detect the project's primary language and inject language-specific review patterns (C++ aggregate init readability, Go error handling, Rust ownership) into Codex prompts.
- **Auto-fix mode** — fully autonomous review-and-fix cycle for vibe coding workflows, requiring zero user interaction until completion.

## Uninstall

Remove the plugin from Claude Code:

```
/plugin uninstall xreview
```

Then clean up the binary and cached data:

```bash
# Remove binary (check which location applies)
rm "$(which xreview)"

# Remove version cache
rm -rf ~/.cache/xreview

# Remove session data (optional)
# Each review session creates a .xreview/ folder in your project root.
# Normally xreview asks to clean it up at the end of a review.
# If you skipped that step, delete it manually:
rm -rf /path/to/your/project/.xreview
```

## License

MIT License — see [LICENSE](LICENSE) for details.

## Support

- **Issues**: https://github.com/davidleitw/xreview/issues
