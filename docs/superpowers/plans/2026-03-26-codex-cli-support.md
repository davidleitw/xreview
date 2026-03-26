# Codex CLI Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let Codex CLI users install and use xreview as a skill, alongside the existing Claude Code support.

**Architecture:** Add a Codex-specific SKILL.md at `.agents/skills/xreview/SKILL.md` (independent copy, not symlink). Add `.codex/INSTALL.md` that Codex agents follow to install the binary and skill files. Update both READMEs to reflect multi-host support.

**Tech Stack:** Markdown, shell (install.sh already exists)

---

### Task 1: Create Codex SKILL.md

The Codex version of the skill. Same workflow as `skills/review/SKILL.md` but adapted for Codex CLI differences.

**Files:**
- Create: `.agents/skills/xreview/SKILL.md`
- Reference: `skills/review/SKILL.md` (Claude Code version, do not modify)
- Reference: `skills/review/reference.md` (XML schema, shared by both)

**Differences from Claude Code version:**
1. Remove `allowed-tools` from frontmatter (Codex doesn't support it)
2. Remove `argument-hint` from frontmatter (Codex doesn't support it)
3. Replace all "Claude Code" references with "you" or generic agent language
4. Remove `AskUserQuestion` — Codex agents ask users directly in conversation
5. Keep all workflow steps, XML parsing, verification logic identical
6. Reference `reference.md` via a note that it's at the repo URL (Codex skill is installed to `~/.agents/skills/xreview/`, not in the repo)

- [ ] **Step 1: Create the Codex SKILL.md**

Copy `skills/review/SKILL.md` and apply these changes:

Frontmatter — replace:
```yaml
---
name: xreview
description: >
  MANDATORY for ALL code review requests. When the user asks to "review", "code review",
  "check code", "找 bug", "review 程式碼", or any variation of reviewing code for bugs,
  security issues, or quality — you MUST use this skill. Do NOT read files and review
  them yourself. This skill delegates review to Codex (a separate AI reviewer) via the
  xreview CLI, enabling three-party review (Codex reviews, you verify, user decides).
  Default mode is review-only: present all findings, let user discuss, then fix on demand.
---
```

No `allowed-tools` or `argument-hint`.

Body changes (apply throughout):
- "Claude Code" → "you" (when referring to the host agent)
- "Claude Code (host)" → "Host Agent" (in architecture descriptions)
- "you Claude Code (verifier)" → "you (verifier)"
- Remove any mention of `AskUserQuestion` tool — just say "ask the user"
- The install step (Step 0, xreview not found): change `curl ... | bash` install to just tell the user to run it, since Codex may not auto-approve piping to bash
- XML schema reference: change `See [reference.md](reference.md)` to `See https://github.com/davidleitw/xreview/blob/master/skills/review/reference.md`

- [ ] **Step 2: Verify the SKILL.md**

Manually check:
- No remaining "Claude Code" references (except in the description of the three-party model where it says "Codex reviews")
- No `allowed-tools` or `argument-hint` in frontmatter
- No `AskUserQuestion` references
- All xreview CLI commands are identical to the Claude Code version
- YAML frontmatter is valid

- [ ] **Step 3: Commit**

```bash
git add .agents/skills/xreview/SKILL.md
git commit -m "feat: add Codex CLI skill definition"
```

---

### Task 2: Create .codex/INSTALL.md

The install instructions that a Codex agent reads and follows when the user pastes the install URL.

**Files:**
- Create: `.codex/INSTALL.md`
- Reference: `scripts/install.sh` (existing binary installer)

**Design:**
- Step 1: Install the xreview binary via existing `install.sh`
- Step 2: Download SKILL.md from GitHub to `~/.agents/skills/xreview/`
- Step 3: Verify installation with `xreview version`
- Codex CLI needs `codex` to already be installed (it's the host), so no need to check for codex

- [ ] **Step 1: Create .codex/INSTALL.md**

```markdown
# xreview Installation for Codex CLI

Follow these steps to install xreview.

## Step 1: Install xreview binary

Run:
```bash
curl -fsSL https://raw.githubusercontent.com/davidleitw/xreview/master/scripts/install.sh | bash
```

If this fails, tell the user to download manually from https://github.com/davidleitw/xreview/releases

## Step 2: Install xreview skill

Run:
```bash
mkdir -p ~/.agents/skills/xreview
curl -fsSL -o ~/.agents/skills/xreview/SKILL.md https://raw.githubusercontent.com/davidleitw/xreview/master/.agents/skills/xreview/SKILL.md
```

## Step 3: Verify

Run: `xreview version`

If the command succeeds, tell the user:
- xreview is installed and ready
- Restart Codex to pick up the new skill
- They can then ask "review my code" to start a review

If it fails, check that `~/.local/bin` is in PATH and suggest adding it.
```

- [ ] **Step 2: Commit**

```bash
git add .codex/INSTALL.md
git commit -m "feat: add Codex CLI install instructions"
```

---

### Task 3: Update English README

**Files:**
- Modify: `README.md`

Three changes:
1. Title/subtitle — make it multi-host
2. Add Codex CLI installation section
3. Architecture diagram — generalize host label

- [ ] **Step 1: Update title line**

Line 3: change
```
Agent-native code review engine for Claude Code, powered by Codex.
```
to
```
Agent-native code review engine for Claude Code and Codex CLI, powered by Codex.
```

- [ ] **Step 2: Add Codex CLI install section**

After the existing `### Claude Code` section (after line 33), add:

```markdown
### Codex CLI

Paste this to your Codex CLI session:

```
Fetch and follow instructions from https://raw.githubusercontent.com/davidleitw/xreview/master/.codex/INSTALL.md
```

Or install manually:

```bash
# Install binary
curl -fsSL https://raw.githubusercontent.com/davidleitw/xreview/master/scripts/install.sh | bash

# Install skill
mkdir -p ~/.agents/skills/xreview
curl -fsSL -o ~/.agents/skills/xreview/SKILL.md https://raw.githubusercontent.com/davidleitw/xreview/master/.agents/skills/xreview/SKILL.md
```
```

- [ ] **Step 3: Update architecture diagram**

Line 147: change `Claude Code (host)` to `Host Agent (Claude Code / Codex CLI)` in the ASCII diagram. Keep all other lines the same — the data flow is identical regardless of host.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add Codex CLI installation to README"
```

---

### Task 4: Update Chinese README

**Files:**
- Modify: `docs/README.zh-TW.md`

Mirror the English README changes for consistency.

- [ ] **Step 1: Update title line**

Line 3: change
```
Agent-native 程式碼審查引擎，專為 Claude Code 設計，由 Codex 驅動。
```
to
```
Agent-native 程式碼審查引擎，支援 Claude Code 與 Codex CLI，由 Codex 驅動。
```

- [ ] **Step 2: Add Codex CLI install section**

After the existing `### Claude Code` section (after line 33), add:

```markdown
### Codex CLI

將以下指令貼到 Codex CLI 對話中：

```
Fetch and follow instructions from https://raw.githubusercontent.com/davidleitw/xreview/master/.codex/INSTALL.md
```

或手動安裝：

```bash
# 安裝 binary
curl -fsSL https://raw.githubusercontent.com/davidleitw/xreview/master/scripts/install.sh | bash

# 安裝 skill
mkdir -p ~/.agents/skills/xreview
curl -fsSL -o ~/.agents/skills/xreview/SKILL.md https://raw.githubusercontent.com/davidleitw/xreview/master/.agents/skills/xreview/SKILL.md
```
```

- [ ] **Step 3: Update architecture diagram**

Line 147: change `Claude Code (host)` to `Host Agent (Claude Code / Codex CLI)` — same as English version.

- [ ] **Step 4: Commit**

```bash
git add docs/README.zh-TW.md
git commit -m "docs: 新增 Codex CLI 安裝說明至中文 README"
```
