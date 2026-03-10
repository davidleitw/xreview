# Claude Code Skill & Plugin Practical Reference

How to create skills, register slash commands, build plugins, and hook into
Claude Code's lifecycle. No conceptual explanations — only syntax and examples.

**Official docs:**
- https://code.claude.com/docs/en/skills.md
- https://code.claude.com/docs/en/plugins.md
- https://code.claude.com/docs/en/plugins-reference.md
- https://code.claude.com/docs/en/hooks.md
- https://code.claude.com/docs/en/permissions.md

---

## 1. Creating a `/` Slash Command (Skill)

A skill = a directory with a `SKILL.md` file.

```
.claude/skills/deploy/SKILL.md    -->  /deploy
~/.claude/skills/hello/SKILL.md   -->  /hello (global)
```

Minimal example:

```yaml
---
name: deploy
description: Deploy the application to staging or production
---

Deploy $ARGUMENTS using our CI/CD pipeline.

1. Run tests: `npm test`
2. Build: `npm run build`
3. Deploy: `./scripts/deploy.sh $0`
```

Usage: `/deploy staging`

---

## 2. SKILL.md Frontmatter — Complete Field Reference

```yaml
---
# Identity
name: my-skill                  # /slash-command name. Lowercase, numbers, hyphens. Max 64 chars.
                                # Omit to use directory name.
description: >                  # Claude reads this to decide when to auto-invoke.
  What this does and when.      # Omit to use first paragraph of content.

# Arguments
argument-hint: "[file] [mode]"  # Shown in autocomplete. Cosmetic only.

# Permissions
allowed-tools: Bash(npm *), Read, Grep   # Tools allowed without asking user.

# Invocation control
disable-model-invocation: false  # true = only human can /invoke. Claude cannot auto-trigger.
user-invocable: true             # false = only Claude can invoke. Hidden from / menu.

# Execution
model: opus                      # Override model when active.
context: fork                    # "fork" = isolated subagent (no conversation history).
agent: Explore                   # Subagent type when context=fork.
                                 # Options: Explore, Plan, general-purpose, or custom agent name.
---
```

### Invocation Control Matrix

| Setting | Human /invoke | Claude auto-invoke | Description in context |
|---|---|---|---|
| (default) | Yes | Yes | Yes |
| `disable-model-invocation: true` | Yes | No | No |
| `user-invocable: false` | No | Yes | Yes |

---

## 3. String Substitutions

Use these anywhere in SKILL.md content:

| Variable | Value | Example input | Result |
|---|---|---|---|
| `$ARGUMENTS` | All args as one string | `/review src/a.go thorough` | `"src/a.go thorough"` |
| `$ARGUMENTS[0]` or `$0` | First arg | same | `"src/a.go"` |
| `$ARGUMENTS[1]` or `$1` | Second arg | same | `"thorough"` |
| `${CLAUDE_SESSION_ID}` | Session UUID | | `"abc-123-def"` |
| `${CLAUDE_SKILL_DIR}` | Skill directory absolute path | | `"/home/user/.claude/skills/review"` |
| `${CLAUDE_PLUGIN_ROOT}` | Plugin root absolute path (plugins only) | | `"/home/user/.claude/plugins/my-plugin"` |

If `$ARGUMENTS` is not present in content, args are appended as `ARGUMENTS: <value>`.

---

## 4. Dynamic Context Injection

Run shell commands **before** sending skill to Claude. Output replaces the placeholder.

```yaml
---
name: pr-summary
---

Current diff:
!`git diff HEAD~1`

Recent commits:
!`git log --oneline -5`

Review based on the above.
```

Claude receives the actual output, not the commands.

---

## 5. allowed-tools Syntax

Comma-separated. Case-sensitive.

### Tool Names

`Read`, `Edit`, `Write`, `Bash`, `Grep`, `Glob`, `WebFetch`, `WebSearch`,
`Agent`, `Skill`, `AskUserQuestion`, `mcp__<server>__<tool>`

### Bash Patterns

```yaml
allowed-tools: Bash(xreview *)        # "xreview" + space + anything
allowed-tools: Bash(npm run *)        # "npm run" + anything
allowed-tools: Bash(*)               # all bash commands
allowed-tools: Bash                  # also all bash commands
```

Word boundary: `Bash(npm *)` matches `npm test` but NOT `npms`.
No word boundary: `Bash(npm*)` matches both.

### Combined

```yaml
allowed-tools: Bash(xreview *), Bash(go install *), Bash(which *), Read, AskUserQuestion
```

---

## 6. Skill Discovery

### Locations (priority order, highest first)

1. Enterprise (managed settings)
2. Personal: `~/.claude/skills/<name>/SKILL.md`
3. Project: `.claude/skills/<name>/SKILL.md`
4. Plugin: `<plugin>/skills/<name>/SKILL.md` (namespaced as `plugin-name:skill-name`)

### Monorepo Auto-Discovery

Working in `packages/frontend/` discovers:
- `.claude/skills/` (project root)
- `packages/frontend/.claude/skills/` (nested)

### Context Budget

- 2% of context window for all skill descriptions combined
- Fallback: 16,000 chars if 2% is too small
- Override: `SLASH_COMMAND_TOOL_CHAR_BUDGET=32000`
- If exceeded, some skills are excluded. Check with `/context`

### Live Reload

Skills in `.claude/skills/` auto-reload on change. No restart needed.

---

## 7. Plugin Structure

A plugin bundles skills + hooks + agents + MCP servers into one distributable unit.

```
my-plugin/
  .claude-plugin/          # REQUIRED directory. Contains ONLY plugin.json.
    plugin.json            # REQUIRED manifest.
  skills/                  # Skills (auto-discovered)
    hello/
      SKILL.md
  hooks/                   # Hooks
    hooks.json
  agents/                  # Agent definitions
    reviewer.md
  commands/                # Additional commands
    deploy.md
  scripts/                 # Supporting scripts
    install.sh
  .mcp.json                # MCP server definitions
  .lsp.json                # LSP server definitions
  settings.json            # Default settings (only "agent" key)
```

**CRITICAL**: Everything goes at the plugin root. `.claude-plugin/` contains
ONLY `plugin.json`.

---

## 8. plugin.json — Complete Schema

```json
{
  "name": "my-plugin",                            // REQUIRED. Kebab-case. Namespace prefix.
  "version": "1.0.0",                             // Semver.
  "description": "What this plugin does",          // Shown in plugin manager.
  "author": {
    "name": "Author",
    "email": "a@b.com",
    "url": "https://github.com/author"
  },
  "homepage": "https://example.com",
  "repository": "https://github.com/author/plugin",
  "license": "MIT",
  "keywords": ["review", "codex"],

  // Component paths — all relative, must start with "./"
  "skills": "./skills/",                           // Skill directory
  "hooks": "./hooks/hooks.json",                   // Hook config file
  "commands": ["./commands/deploy.md"],             // Command files
  "agents": "./agents/",                            // Agent definitions
  "mcpServers": "./.mcp.json",                      // MCP server config
  "lspServers": "./.lsp.json",                      // LSP server config
  "outputStyles": "./styles/"                       // Output styles
}
```

### Namespacing

Plugin skills are always namespaced:

```
my-plugin/skills/hello/SKILL.md  -->  /my-plugin:hello
my-plugin/skills/world/SKILL.md  -->  /my-plugin:world
```

Cannot use short names. Plugin name from `plugin.json` is the prefix.

---

## 9. Hooks

### File Format

Location: any `.json` file referenced by `plugin.json` `hooks` field,
or project-level `.claude/settings.json`.

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/validate.sh"
          }
        ]
      }
    ]
  }
}
```

### Available Events

| Event | Fires when | Matcher matches on |
|---|---|---|
| `SessionStart` | Session begins/resumes | `startup`, `resume`, `clear`, `compact` |
| `UserPromptSubmit` | User sends message | (no matcher) |
| `PreToolUse` | Before tool runs | Tool name: `Bash`, `Edit`, `Write`, etc. |
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

### Matcher Syntax

Regex pattern matching against the matcher field:

```json
{"matcher": "Edit|Write"}           // Matches Edit OR Write
{"matcher": "Bash"}                 // Matches Bash
{"matcher": "mcp__github__.*"}      // All github MCP tools
{"matcher": ""}                     // Matches everything
```

### Hook Types

| Type | Description |
|---|---|
| `command` | Shell command. Receives JSON on stdin. |
| `http` | POST to URL with JSON payload. |
| `prompt` | Single LLM call. |
| `agent` | Multi-turn agent with tools. |

### Hook Input (stdin for command hooks)

```json
{
  "session_id": "abc123",
  "cwd": "/path/to/project",
  "hook_event_name": "PostToolUse",
  "tool_name": "Bash",
  "tool_input": {
    "command": "npm test"
  }
}
```

### Hook Exit Codes

| Exit | Effect |
|---|---|
| 0 | Action proceeds. Stdout added to Claude's context. |
| 2 | Action **BLOCKED**. Stderr shown to Claude as feedback. |
| Other | Action proceeds. Stderr logged, not shown to Claude. |

---

## 10. Permissions in settings.json

```json
{
  "permissions": {
    "allow": [
      "Bash(xreview *)",
      "Read(/src/**)",
      "WebFetch(domain:github.com)",
      "Skill(commit)",
      "Skill(review-pr *)"
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
- `**` — recursive, `*` — single directory

**Restrict all skills:**

```json
{"permissions": {"deny": ["Skill"]}}
```

**Allow specific skills only:**

```json
{"permissions": {"allow": ["Skill(commit)", "Skill(review-pr *)"]}}
```

---

## 11. Plugin Installation & Testing

```bash
# Local development (no install, instant)
claude --plugin-dir ./my-plugin

# Install
claude plugin install <name>                   # From marketplace
claude plugin install <name>@<marketplace>     # Specific marketplace
claude plugin install <name> --scope user      # Personal (default)
claude plugin install <name> --scope project   # Shared via git
claude plugin install <name> --scope local     # Gitignored, this machine

# Manage
claude plugin enable <name>
claude plugin disable <name>
claude plugin update <name>
claude plugin uninstall <name>

# Reload after changes (in-session)
/reload-plugins
```

---

## 12. Cookbook: Skill That Calls an External Binary

### Goal

Create `/review` that calls a Go binary (`xreview`) and presents results.

### Project-Level Skill

```
.claude/skills/review/
  SKILL.md
  reference.md
```

```yaml
# SKILL.md
---
name: review
description: >
  Run code review using xreview CLI after completing a task.
allowed-tools: Bash(xreview *), Bash(go install *), Bash(which *), Read, AskUserQuestion
argument-hint: [files]
---

## Step 0: Check binary

1. `which xreview`
2. If missing: `go install github.com/davidleitw/xreview@latest`
3. If go missing: tell user to install Go or download from GitHub Releases.

## Step 1: Run review

`xreview review --files $ARGUMENTS --context "task context here"`

## Step 2: Present results

Parse XML output. Show findings in plain language.
See [reference.md](reference.md) for XML schema.
```

### Plugin-Packaged Skill

```
xreview-plugin/
  .claude-plugin/
    plugin.json
  skills/
    review/
      SKILL.md
      reference.md
  scripts/
    install-binary.sh
```

```json
// plugin.json
{
  "name": "xreview",
  "version": "0.1.0",
  "description": "Code review powered by Codex",
  "skills": "./skills/"
}
```

```yaml
# skills/review/SKILL.md
---
name: review
description: Code review using Codex. Use after completing implementation tasks.
allowed-tools: Bash(xreview *), Bash(${CLAUDE_PLUGIN_ROOT}/scripts/*), Read, AskUserQuestion
---

## Step 0: Install
`${CLAUDE_PLUGIN_ROOT}/scripts/install-binary.sh`

## Step 1: Review
`xreview review --files $ARGUMENTS`
```

Invocation: `/xreview:review src/auth.go`

---

## 13. Cookbook: Hook That Auto-Suggests Review

```json
// hooks/hooks.json
{
  "hooks": {
    "Stop": [
      {
        "matcher": "auto",
        "hooks": [
          {
            "type": "command",
            "command": "echo 'Consider running /xreview:review if you just completed a task.'"
          }
        ]
      }
    ]
  }
}
```

The `Stop` event fires when Claude finishes a response. Exit 0 + stdout message
gets added to Claude's context, nudging it to suggest a review.

---

## 14. Cookbook: Subagent Skill (Isolated Context)

```yaml
---
name: deep-analysis
description: Thorough codebase analysis
context: fork
agent: Explore
---

Analyze $ARGUMENTS thoroughly. Find all usages, read implementations,
and summarize the architecture.
```

Runs in an isolated subagent with read-only tools (Explore agent).
Does not pollute the main conversation context.

---

## 15. Quick Reference Card

| I want to... | Do this |
|---|---|
| Create a slash command | `.claude/skills/<name>/SKILL.md` |
| Make it global | `~/.claude/skills/<name>/SKILL.md` |
| Allow bash without prompts | `allowed-tools: Bash(my-tool *)` |
| Pass user input | `$ARGUMENTS`, `$0`, `$1` |
| Reference skill directory | `${CLAUDE_SKILL_DIR}` |
| Inject command output at load time | `` !`git status` `` |
| Prevent Claude from auto-triggering | `disable-model-invocation: true` |
| Hide from / menu (background skill) | `user-invocable: false` |
| Run in isolated context | `context: fork` + `agent: Explore` |
| Bundle as plugin | Add `.claude-plugin/plugin.json` |
| Test plugin locally | `claude --plugin-dir ./my-plugin` |
| Install plugin | `claude plugin install <name>` |
| Hook into tool execution | `hooks.json` with `PreToolUse`/`PostToolUse` |
| Block an action from hook | Exit code 2, message on stderr |
