# xreview Plugin 化 + GitHub Actions Release Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 將 xreview 包裝成 Claude Code plugin，加入 GitHub Actions cross-compile release workflow，讓使用者能透過 marketplace 安裝並自動下載 pre-built binary。

**Architecture:** 在現有 repo 內新增 `plugin/` 目錄作為 Claude Code plugin，`scripts/install.sh` 從 GitHub Releases 下載 binary。repo 根目錄的 `.claude-plugin/marketplace.json` 讓使用者透過 `/plugin marketplace add davidleitw/xreview` 安裝。GitHub Actions 在 push `v*` tag 時 cross-compile 四個平台的 binary 並上傳到 Release。

**Tech Stack:** Go, GitHub Actions, Shell script (bash), Claude Code Plugin System

---

## Chunk 1: Plugin 結構 + Install Script

### Task 1: 建立 Plugin Manifest

**Files:**
- Create: `plugin/.claude-plugin/plugin.json`

- [ ] **Step 1: 建立目錄和 plugin.json**

```bash
mkdir -p plugin/.claude-plugin
```

寫入 `plugin/.claude-plugin/plugin.json`：

```json
{
  "name": "xreview",
  "version": "0.1.0",
  "description": "AI-powered code review using Codex — three-party consensus between Codex, Claude Code, and you",
  "author": {
    "name": "davidleitw",
    "url": "https://github.com/davidleitw"
  },
  "repository": "https://github.com/davidleitw/xreview",
  "license": "MIT",
  "keywords": ["code-review", "codex", "xreview"],
  "skills": "./skills/"
}
```

- [ ] **Step 2: Commit**

```bash
git add plugin/.claude-plugin/plugin.json
git commit -m "feat: add plugin manifest for Claude Code plugin system"
```

---

### Task 2: 建立 Install Script

**Files:**
- Create: `plugin/scripts/install.sh`

- [ ] **Step 1: 寫 install.sh**

寫入 `plugin/scripts/install.sh`：

```bash
#!/usr/bin/env bash
set -euo pipefail

REPO="davidleitw/xreview"
INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="xreview"

# --- Detect OS ---
OS="$(uname -s)"
case "${OS}" in
  Linux*)  OS_NAME="linux" ;;
  Darwin*) OS_NAME="darwin" ;;
  *)
    echo "Error: Unsupported OS: ${OS}" >&2
    echo "Supported: Linux, macOS" >&2
    exit 1
    ;;
esac

# --- Detect Architecture ---
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64)  ARCH_NAME="amd64" ;;
  amd64)   ARCH_NAME="amd64" ;;
  aarch64) ARCH_NAME="arm64" ;;
  arm64)   ARCH_NAME="arm64" ;;
  *)
    echo "Error: Unsupported architecture: ${ARCH}" >&2
    echo "Supported: x86_64/amd64, aarch64/arm64" >&2
    exit 1
    ;;
esac

ASSET_NAME="${BINARY_NAME}-${OS_NAME}-${ARCH_NAME}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"

echo "Detected: ${OS_NAME}/${ARCH_NAME}"
echo "Downloading ${BINARY_NAME} from GitHub Releases..."

# --- Create install directory ---
mkdir -p "${INSTALL_DIR}"

# --- Download ---
HTTP_CODE=""
if command -v curl &>/dev/null; then
  HTTP_CODE=$(curl -fSL -o "${INSTALL_DIR}/${BINARY_NAME}" -w "%{http_code}" "${DOWNLOAD_URL}" 2>/dev/null) || true
elif command -v wget &>/dev/null; then
  if wget -q -O "${INSTALL_DIR}/${BINARY_NAME}" "${DOWNLOAD_URL}" 2>/dev/null; then
    HTTP_CODE="200"
  fi
fi

if [ "${HTTP_CODE}" = "200" ] && [ -f "${INSTALL_DIR}/${BINARY_NAME}" ] && [ -s "${INSTALL_DIR}/${BINARY_NAME}" ]; then
  chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
  echo "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
else
  # Clean up partial download
  rm -f "${INSTALL_DIR}/${BINARY_NAME}"
  echo "Download failed. Trying go install as fallback..." >&2

  if command -v go &>/dev/null; then
    go install "github.com/${REPO}/cmd/xreview@latest"
    echo "Installed via go install."
  else
    echo "Error: Could not download binary and Go is not installed." >&2
    echo "Please download manually from: https://github.com/${REPO}/releases" >&2
    exit 1
  fi
fi

# --- Verify PATH ---
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo ""
    echo "Warning: ${INSTALL_DIR} is not in your PATH."
    echo "Add this to your shell profile (~/.bashrc or ~/.zshrc):"
    echo "  export PATH=\"${INSTALL_DIR}:\${PATH}\""
    ;;
esac

# --- Verify installation ---
if command -v "${BINARY_NAME}" &>/dev/null; then
  echo ""
  "${BINARY_NAME}" version
else
  echo ""
  echo "${BINARY_NAME} installed to ${INSTALL_DIR}/${BINARY_NAME}"
  echo "Run '${BINARY_NAME} version' after adding ${INSTALL_DIR} to PATH."
fi
```

- [ ] **Step 2: 設為可執行**

```bash
chmod +x plugin/scripts/install.sh
```

- [ ] **Step 3: Commit**

```bash
git add plugin/scripts/install.sh
git commit -m "feat: add install script for downloading xreview binary"
```

---

### Task 3: 建立 Plugin Skill（SKILL.md + reference.md）

**Files:**
- Create: `plugin/skills/review/SKILL.md`
- Copy: `plugin/skills/review/reference.md`（從 `.claude/skills/xreview/reference.md` 複製）

- [ ] **Step 1: 複製 reference.md**

```bash
mkdir -p plugin/skills/review
cp .claude/skills/xreview/reference.md plugin/skills/review/reference.md
```

- [ ] **Step 2: 寫 plugin 版 SKILL.md**

寫入 `plugin/skills/review/SKILL.md`：

```yaml
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
```

- [ ] **Step 3: Commit**

```bash
git add plugin/skills/review/SKILL.md plugin/skills/review/reference.md
git commit -m "feat: add plugin skill for xreview review workflow"
```

---

### Task 4: 建立 Marketplace 定義

**Files:**
- Create: `.claude-plugin/marketplace.json`

- [ ] **Step 1: 建立 marketplace.json**

```bash
mkdir -p .claude-plugin
```

寫入 `.claude-plugin/marketplace.json`：

```json
{
  "name": "xreview-marketplace",
  "owner": {
    "name": "davidleitw",
    "email": "davidleitw@gmail.com"
  },
  "metadata": {
    "description": "xreview — AI-powered code review plugin for Claude Code"
  },
  "plugins": [
    {
      "name": "xreview",
      "source": "./plugin",
      "description": "AI-powered code review using Codex — three-party consensus between Codex, Claude Code, and you",
      "homepage": "https://github.com/davidleitw/xreview",
      "keywords": ["code-review", "codex", "xreview"]
    }
  ]
}
```

- [ ] **Step 2: 更新 .gitignore**

在 `.gitignore` 末尾確認不會忽略 `.claude-plugin/` 目錄。當前 `.gitignore` 沒有排除它，不需要改動。確認即可。

- [ ] **Step 3: Commit**

```bash
git add .claude-plugin/marketplace.json
git commit -m "feat: add marketplace definition for plugin distribution"
```

---

## Chunk 2: GitHub Actions Release Workflow

### Task 5: 建立 Release Workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: 建立 workflow 檔案**

```bash
mkdir -p .github/workflows
```

寫入 `.github/workflows/release.yml`：

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run tests
        run: go test ./...

      - name: Extract version from tag
        id: version
        run: echo "VERSION=${GITHUB_REF_NAME#v}" >> "$GITHUB_OUTPUT"

      - name: Cross-compile binaries
        run: |
          VERSION="${{ steps.version.outputs.VERSION }}"
          LDFLAGS="-X github.com/davidleitw/xreview/internal/version.Version=${VERSION}"

          GOOS=linux  GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o dist/xreview-linux-amd64  ./cmd/xreview
          GOOS=linux  GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o dist/xreview-linux-arm64  ./cmd/xreview
          GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o dist/xreview-darwin-amd64 ./cmd/xreview
          GOOS=darwin GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o dist/xreview-darwin-arm64 ./cmd/xreview

      - name: Generate checksums
        run: |
          cd dist
          sha256sum xreview-* > checksums.txt

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          generate_release_notes: true
          files: |
            dist/xreview-linux-amd64
            dist/xreview-linux-arm64
            dist/xreview-darwin-amd64
            dist/xreview-darwin-arm64
            dist/checksums.txt
```

- [ ] **Step 2: 加入 dist/ 到 .gitignore**

在 `.gitignore` 末尾加入：

```
# Release build output
dist/
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml .gitignore
git commit -m "feat: add GitHub Actions release workflow for cross-platform builds"
```

---

### Task 6: 更新 Makefile 加入 cross-compile target

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: 在 Makefile 末尾加入 cross-compile target**

在 `Makefile` 的 `clean:` target 後加入：

```makefile

cross-compile:
	@mkdir -p dist/
	GOOS=linux  GOARCH=amd64 go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o dist/xreview-linux-amd64  ./cmd/xreview
	GOOS=linux  GOARCH=arm64 go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o dist/xreview-linux-arm64  ./cmd/xreview
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o dist/xreview-darwin-amd64 ./cmd/xreview
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o dist/xreview-darwin-arm64 ./cmd/xreview
```

並更新 `.PHONY` 行：

```makefile
.PHONY: build test lint install clean cross-compile
```

- [ ] **Step 2: 本地測試 cross-compile**

```bash
make cross-compile VERSION=0.1.0
ls -la dist/
```

Expected: 四個 binary 檔案在 `dist/` 目錄。

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "feat: add cross-compile target to Makefile"
```

---

## Chunk 3: 本地測試 Plugin + 清理

### Task 7: 用 --plugin-dir 本地測試 plugin

**Files:** 無新增（驗證性步驟）

- [ ] **Step 1: 用 --plugin-dir 載入 plugin**

```bash
claude --plugin-dir ./plugin
```

進入後確認：
- `/xreview:review` 出現在可用 skill 列表中
- 執行 `/xreview:review` 看 Step 0 是否正確嘗試偵測/安裝 xreview

- [ ] **Step 2: 測試 install.sh 獨立運行**

```bash
# 先移除 xreview（如果在 PATH 中）
# 然後測試 install script
bash plugin/scripts/install.sh
```

確認：
- 偵測到正確的 OS/arch
- 下載失敗時 fallback 到 go install（因為還沒有 release）
- 安裝成功後能執行 `xreview version`

- [ ] **Step 3: 測試 marketplace 本地安裝**

```bash
# 在 Claude Code session 中
/plugin marketplace add ./
/plugin install xreview@xreview-marketplace
```

確認 plugin 安裝成功且 `/xreview:review` 可用。

---

### Task 8: 移除 repo 根目錄的 xreview binary

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: 確認根目錄的 xreview binary 不應被追蹤**

```bash
ls -la xreview
```

目前 repo 根目錄有一個 6.7M 的 `xreview` binary。檢查它是否被 git 追蹤：

```bash
git ls-files xreview
```

如果被追蹤，從 git 移除（保留本地檔案）：

```bash
git rm --cached xreview
```

- [ ] **Step 2: 在 .gitignore 加入根目錄 binary**

在 `.gitignore` 加入：

```
# Built binary at root (use bin/ or dist/ instead)
/xreview
```

- [ ] **Step 3: Commit**

```bash
git add .gitignore
git commit -m "chore: stop tracking root xreview binary, add to gitignore"
```

---

### Task 9: 首次 Release 測試

**Files:** 無新增（驗證性步驟）

- [ ] **Step 1: 確認所有測試通過**

```bash
go test ./...
```

Expected: 全部 PASS。

- [ ] **Step 2: 打 tag 並 push 觸發 release**

```bash
git tag v0.1.0
git push origin master --tags
```

- [ ] **Step 3: 確認 GitHub Release**

在 GitHub 上確認：
- Release `v0.1.0` 已建立
- 四個 binary + checksums.txt 已上傳
- Release notes 已自動產生

- [ ] **Step 4: 測試 install.sh 從 release 下載**

```bash
# 移除現有的 xreview
rm -f ~/.local/bin/xreview

# 重新用 install script 下載
bash plugin/scripts/install.sh

# 確認版本
xreview version
```

Expected: 顯示 `0.1.0`。

---

## 完整檔案清單

| 操作 | 路徑 |
|------|------|
| Create | `plugin/.claude-plugin/plugin.json` |
| Create | `plugin/scripts/install.sh` |
| Create | `plugin/skills/review/SKILL.md` |
| Copy | `plugin/skills/review/reference.md` |
| Create | `.claude-plugin/marketplace.json` |
| Create | `.github/workflows/release.yml` |
| Modify | `Makefile` |
| Modify | `.gitignore` |
