# xreview

Agent-native code review engine for Claude Code, powered by Codex.

xreview delegates code review to Codex (a separate AI model) so Claude Code gets an independent second opinion. It orchestrates a three-party review loop: **Codex reviews, Claude Code fixes, you decide.**

**[中文版 README](docs/README.zh-TW.md)**

## How It Works

When you ask Claude Code to review your code, the xreview skill takes over:

1. **Codex reviews** your code and reports findings (bugs, security issues, logic errors)
2. **Claude Code analyzes** each finding — explains the trigger, root cause, and impact
3. **You decide** — obvious fixes are applied automatically; ambiguous ones are presented as options
4. **Codex verifies** the fixes in a follow-up round, may find new issues or reopen dismissed ones
5. **Repeat** until all parties agree (or 5 rounds max)

This isn't Claude Code reviewing its own work. It's a genuinely independent review from a different model with different strengths.

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

- **Simple fix, one obvious solution** — Claude Code applies it directly and tells you what it did
- **Multiple valid approaches** — Claude Code presents options with a recommendation; you pick
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

## Building from Source

```bash
git clone https://github.com/davidleitw/xreview.git
cd xreview
go build -o xreview ./cmd/xreview/
```

## Architecture

```
Claude Code (host)          xreview (CLI)           Codex (reviewer)
     |                          |                        |
     |-- /xreview skill ------->|                        |
     |                          |-- codex exec --------->|
     |                          |<-- findings (JSON) ----|
     |<-- findings (XML) ------|                        |
     |                          |                        |
     |  [fix code]              |                        |
     |                          |                        |
     |-- resume --------------->|                        |
     |                          |-- codex resume ------->|
     |                          |<-- verify (JSON) ------|
     |<-- verify (XML) --------|                        |
```

- xreview outputs XML on stdout for Claude Code skill consumption
- Internal state stored as JSON in `.xreview/sessions/`
- Codex integration via `codex exec` with `--output-schema` for structured responses
- Multi-round: codex session resume via `--resume <session-id>`

## Future Work

- **Language-aware review context** — detect the project's primary language and pass language-specific best practices (e.g., Go error handling patterns, Rust ownership rules, Python type safety) as additional context to Codex, so reviews are informed by the idioms and conventions of the language being reviewed.

## License

MIT License — see [LICENSE](LICENSE) for details.

## Support

- **Issues**: https://github.com/davidleitw/xreview/issues
