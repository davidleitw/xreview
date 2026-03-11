# Claude Code Plugin System 完整指南

> 本文基於 Claude Code 官方文件整理，涵蓋 Plugin、Skill、Hook 的完整架構、開發流程、上架方式與技術細節。
>
> 最後更新：2026-03-11 ｜ 適用版本：Claude Code 1.0.33+

---

## 目錄

- [1. 核心概念：Plugin、Skill、Hook 的關係](#1-核心概念pluginskillhook-的關係)
- [2. Skill 完整指南](#2-skill-完整指南)
- [3. Plugin 完整指南](#3-plugin-完整指南)
- [4. Hook 完整指南](#4-hook-完整指南)
- [5. Marketplace 與上架流程](#5-marketplace-與上架流程)
- [6. 本地開發與測試](#6-本地開發與測試)
- [7. 團隊部署與企業管理](#7-團隊部署與企業管理)
- [8. 參考資料](#8-參考資料)

---

## 1. 核心概念：Plugin、Skill、Hook 的關係

```
Plugin（分發容器）
├── skills/          → slash commands（/plugin-name:skill-name）
├── agents/          → 自定義 subagent
├── hooks/           → 事件驅動的自動化處理
├── commands/        → Markdown 指令（legacy，建議用 skills/）
├── .mcp.json        → MCP server 整合
├── .lsp.json        → LSP server（程式碼智慧提示）
└── settings.json    → 預設設定
```

### 什麼時候用 Standalone Skill vs Plugin？

| 方式 | Skill 名稱 | 適合場景 |
|------|-----------|---------|
| **Standalone**（`.claude/skills/`） | `/hello` | 個人工作流、專案內客製化、快速實驗 |
| **Plugin**（`.claude-plugin/plugin.json`） | `/plugin-name:hello` | 團隊共享、社群發佈、跨專案重複使用 |

**建議**：先用 standalone skill 在 `.claude/` 內快速迭代，準備好分享時再轉成 plugin。

---

## 2. Skill 完整指南

### 2.1 什麼是 Skill？

Skill 是一個 `SKILL.md` 檔案，裡面寫著給 Claude 的指令。Claude 會根據情境自動載入相關 skill，或是由使用者手動以 `/skill-name` 觸發。

Claude Code 的 skill 遵循 [Agent Skills](https://agentskills.io) 開放標準，可跨多種 AI 工具使用。

### 2.2 Skill 存放位置與優先級

| 層級 | 路徑 | 作用範圍 |
|------|------|---------|
| Enterprise | Managed settings | 組織內所有使用者 |
| Personal | `~/.claude/skills/<name>/SKILL.md` | 你所有的專案 |
| Project | `.claude/skills/<name>/SKILL.md` | 僅此專案 |
| Plugin | `<plugin>/skills/<name>/SKILL.md` | 啟用該 plugin 的地方 |

**優先級**：Enterprise > Personal > Project。Plugin skill 使用 `plugin-name:skill-name` namespace，不會與其他層級衝突。

**Monorepo 自動發現**：在 `packages/frontend/` 工作時，Claude Code 也會搜尋 `packages/frontend/.claude/skills/`。

### 2.3 Skill 目錄結構

```
my-skill/
├── SKILL.md           # 主要指令（必要）
├── reference.md       # 詳細 API 文件（需要時載入）
├── examples.md        # 使用範例
└── scripts/
    └── helper.py      # 輔助腳本
```

在 `SKILL.md` 中引用附加檔案：

```markdown
## 補充資源
- 完整 API 細節請參考 [reference.md](reference.md)
- 使用範例請參考 [examples.md](examples.md)
```

> 建議 `SKILL.md` 控制在 500 行以內，詳細資料放到獨立檔案。

### 2.4 SKILL.md Frontmatter 完整欄位

```yaml
---
name: my-skill                    # 顯示名稱，省略則用目錄名。小寫字母、數字、連字號，最多 64 字元
description: >                    # Claude 用此決定何時自動載入。省略則用正文第一段
  這個 skill 做什麼、何時使用
argument-hint: "[issue-number]"   # 自動補全時顯示的提示，純裝飾
disable-model-invocation: false   # true = 只有使用者能 /invoke，Claude 不會自動觸發
user-invocable: true              # false = 只有 Claude 能觸發，從 / 選單隱藏
allowed-tools: Read, Grep, Glob   # 此 skill 啟動時 Claude 可用的工具（不需逐次授權）
model: opus                       # 覆蓋使用的模型
context: fork                     # "fork" = 在隔離的 subagent 中執行
agent: Explore                    # context=fork 時使用的 subagent 類型
hooks:                            # 此 skill 生命週期內的 hooks
  PreToolUse: [...]
---
```

### 2.5 觸發控制矩陣

| Frontmatter 設定 | 使用者可觸發 | Claude 可自動觸發 | 載入時機 |
|-----------------|------------|----------------|---------|
| （預設） | Yes | Yes | Description 常駐 context，觸發時載入全文 |
| `disable-model-invocation: true` | Yes | No | Description 不在 context，使用者觸發時載入 |
| `user-invocable: false` | No | Yes | Description 常駐 context，Claude 觸發時載入 |

### 2.6 參數傳遞機制

當使用者輸入 `/migrate-component SearchBar React Vue` 時：

| 變數 | 值 | 說明 |
|------|-----|------|
| `$ARGUMENTS` | `"SearchBar React Vue"` | 所有參數合成一個字串 |
| `$ARGUMENTS[0]` 或 `$0` | `"SearchBar"` | 第一個參數 |
| `$ARGUMENTS[1]` 或 `$1` | `"React"` | 第二個參數 |
| `$ARGUMENTS[2]` 或 `$2` | `"Vue"` | 第三個參數 |
| `${CLAUDE_SESSION_ID}` | UUID | 當前 session ID |
| `${CLAUDE_SKILL_DIR}` | 絕對路徑 | SKILL.md 所在目錄 |
| `${CLAUDE_PLUGIN_ROOT}` | 絕對路徑 | Plugin 根目錄（僅 plugin skill） |

**機制**：這是**字串替換**，在 skill 內容送給 Claude 之前就完成。

```yaml
---
name: migrate-component
description: 從一個框架遷移元件到另一個
---

將 $0 元件從 $1 遷移到 $2。保留所有現有行為和測試。
```

如果 SKILL.md 中**沒有** `$ARGUMENTS`，系統會自動在末尾附加 `ARGUMENTS: <使用者輸入>`。

### 2.7 動態 Context 注入

用 `` !`command` `` 語法在 skill 載入前執行 shell 指令，輸出替換佔位符：

```yaml
---
name: pr-summary
description: 總結 PR 的變更
context: fork
agent: Explore
allowed-tools: Bash(gh *)
---

## PR 資訊
- PR diff: !`gh pr diff`
- PR 留言: !`gh pr view --comments`
- 變更的檔案: !`gh pr diff --name-only`

## 你的任務
根據以上資訊總結這個 PR...
```

這是**預處理**，Claude 只看到最終結果，不會看到指令本身。

### 2.8 在 Subagent 中執行 Skill

加 `context: fork` 讓 skill 在隔離環境中執行：

```yaml
---
name: deep-research
description: 深入研究某個主題
context: fork
agent: Explore
---

徹底研究 $ARGUMENTS：
1. 用 Glob 和 Grep 找到相關檔案
2. 閱讀並分析程式碼
3. 以具體的檔案引用總結發現
```

`agent` 可用值：`Explore`、`Plan`、`general-purpose`、或任何自訂 subagent 名稱。

### 2.9 內建 Skill

Claude Code 內建以下 skill：

| Skill | 功能 |
|-------|------|
| `/simplify` | 審查最近變更的檔案，找出程式碼重複、品質、效率問題並修復 |
| `/batch <instruction>` | 平行處理大規模跨 codebase 變更，每個單元在獨立 git worktree 中執行 |
| `/debug [description]` | 讀取 session debug log 排查問題 |
| `/loop [interval] <prompt>` | 定期重複執行指令（如每 5 分鐘檢查部署狀態） |
| `/claude-api` | 載入 Claude API 參考資料（支援 Python、TypeScript、Java、Go 等） |

### 2.10 Context 預算

Skill description 會載入 context 讓 Claude 知道有哪些可用。預算為 context window 的 2%（fallback: 16,000 字元）。

可透過環境變數覆蓋：`SLASH_COMMAND_TOOL_CHAR_BUDGET=32000`

---

## 3. Plugin 完整指南

### 3.1 Plugin 目錄結構

```
my-plugin/
├── .claude-plugin/           # metadata 目錄
│   └── plugin.json           # plugin manifest（唯一放這裡的檔案）
├── commands/                 # Markdown 指令（legacy）
├── agents/                   # Subagent 定義
├── skills/                   # Agent Skills（SKILL.md）
│   ├── code-review/
│   │   └── SKILL.md
│   └── pdf-processor/
│       ├── SKILL.md
│       └── scripts/
├── hooks/
│   └── hooks.json            # Hook 設定
├── settings.json             # 預設設定（目前僅支援 "agent" key）
├── .mcp.json                 # MCP server 定義
├── .lsp.json                 # LSP server 設定
├── scripts/                  # 輔助腳本
└── README.md
```

> **重要**：`.claude-plugin/` 裡面**只能放** `plugin.json`。所有其他目錄都放在 plugin 根目錄。

### 3.2 plugin.json 完整 Schema

```json
{
  "name": "my-plugin",                            // 必要。kebab-case，作為 namespace 前綴
  "version": "1.0.0",                             // 語意化版本
  "description": "Plugin 的功能描述",              // 顯示在 plugin manager
  "author": {
    "name": "Author Name",
    "email": "author@example.com",
    "url": "https://github.com/author"
  },
  "homepage": "https://docs.example.com/plugin",
  "repository": "https://github.com/author/plugin",
  "license": "MIT",
  "keywords": ["keyword1", "keyword2"],

  // 元件路徑 — 全部相對路徑，必須以 "./" 開頭
  "commands": ["./custom/commands/special.md"],    // 指令檔案
  "agents": "./custom/agents/",                    // Agent 定義
  "skills": "./custom/skills/",                    // Skill 目錄
  "hooks": "./config/hooks.json",                  // Hook 設定
  "mcpServers": "./mcp-config.json",               // MCP server 設定
  "lspServers": "./.lsp.json",                     // LSP server 設定
  "outputStyles": "./styles/"                      // 輸出風格
}
```

**必要欄位**：只有 `name`。其餘都是選填。

**路徑行為**：自訂路徑是**補充**預設目錄，不是取代。即使你指定了 `"commands": ["./custom/cmd.md"]`，`commands/` 目錄的內容仍會被載入。

**`${CLAUDE_PLUGIN_ROOT}`**：在 hooks、MCP server、腳本中使用此變數來引用 plugin 目錄的絕對路徑，確保安裝後路徑正確。

### 3.3 Namespace 規則

Plugin 內的 skill 一律加上 plugin name 作為 namespace：

```
my-plugin/skills/hello/SKILL.md  →  /my-plugin:hello
my-plugin/skills/world/SKILL.md  →  /my-plugin:world
```

無法使用短名稱。這是為了防止多個 plugin 之間的名稱衝突。

### 3.4 安裝範圍（Scope）

| Scope | 設定檔 | 適用場景 |
|-------|--------|---------|
| `user`（預設） | `~/.claude/settings.json` | 個人 plugin，所有專案可用 |
| `project` | `.claude/settings.json` | 團隊 plugin，透過版本控制共享 |
| `local` | `.claude/settings.local.json` | 專案專用，gitignored |
| `managed` | Managed settings | 管理者部署，唯讀 |

### 3.5 Plugin 快取機制

透過 marketplace 安裝的 plugin 會被複製到 `~/.claude/plugins/cache`。因此：

- Plugin **不能**引用目錄外的檔案（`../shared-utils` 不會工作）
- 如需共享外部檔案，使用 **symlink**（symlink 在複製時會被 follow）
- `--plugin-dir` 直接從檔案系統載入，不受此限制

### 3.6 LSP Server（程式碼智慧提示）

Plugin 可附帶 LSP server 設定，給 Claude 即時的程式碼智慧提示：

```json
// .lsp.json
{
  "go": {
    "command": "gopls",
    "args": ["serve"],
    "extensionToLanguage": {
      ".go": "go"
    }
  }
}
```

官方 marketplace 已提供的 LSP plugin：

| 語言 | Plugin | 需安裝的 binary |
|------|--------|---------------|
| C/C++ | `clangd-lsp` | `clangd` |
| Go | `gopls-lsp` | `gopls` |
| Python | `pyright-lsp` | `pyright-langserver` |
| Rust | `rust-analyzer-lsp` | `rust-analyzer` |
| TypeScript | `typescript-lsp` | `typescript-language-server` |
| Java | `jdtls-lsp` | `jdtls` |
| Kotlin | `kotlin-lsp` | `kotlin-language-server` |
| PHP | `php-lsp` | `intelephense` |
| Swift | `swift-lsp` | `sourcekit-lsp` |
| Lua | `lua-lsp` | `lua-language-server` |
| C# | `csharp-lsp` | `csharp-ls` |

### 3.7 CLI 指令

```bash
# 安裝
claude plugin install <name>@<marketplace>
claude plugin install <name>@<marketplace> --scope project

# 管理
claude plugin enable <name>@<marketplace>
claude plugin disable <name>@<marketplace>
claude plugin update <name>@<marketplace>
claude plugin uninstall <name>@<marketplace>   # 別名：remove, rm

# 驗證
claude plugin validate .

# 在 session 中重新載入（不需重啟）
/reload-plugins
```

---

## 4. Hook 完整指南

### 4.1 什麼是 Hook？

Hook 是在 Claude Code 生命週期特定時間點自動執行的處理程序。它們提供**確定性控制**——確保某些動作一定會發生，而不是依賴 LLM 選擇執行。

### 4.2 Hook 事件完整列表

| 事件 | 觸發時機 | Matcher 過濾對象 |
|------|---------|-----------------|
| `SessionStart` | session 開始或恢復 | `startup`, `resume`, `clear`, `compact` |
| `UserPromptSubmit` | 使用者送出訊息（Claude 處理前） | 無 matcher，每次都觸發 |
| `PreToolUse` | 工具執行**前**，可阻擋 | 工具名稱：`Bash`, `Edit`, `Write` 等 |
| `PostToolUse` | 工具成功執行**後** | 工具名稱 |
| `PostToolUseFailure` | 工具執行**失敗**後 | 工具名稱 |
| `PermissionRequest` | 權限對話框出現時 | 工具名稱 |
| `Notification` | Claude Code 發送通知 | `permission_prompt`, `idle_prompt`, `auth_success`, `elicitation_dialog` |
| `SubagentStart` | subagent 啟動 | agent 類型：`Bash`, `Explore`, `Plan`, 或自訂名稱 |
| `SubagentStop` | subagent 結束 | agent 類型 |
| `Stop` | Claude 完成回應 | `manual`, `auto` |
| `TeammateIdle` | agent team 的隊友即將閒置 | 無 matcher |
| `TaskCompleted` | task 被標記為完成 | 無 matcher |
| `InstructionsLoaded` | CLAUDE.md 或 `.claude/rules/*.md` 被載入 context | — |
| `ConfigChange` | 設定檔在 session 中被變更 | `user_settings`, `project_settings`, `local_settings`, `policy_settings`, `skills` |
| `WorktreeCreate` | worktree 被建立 | 無 matcher |
| `WorktreeRemove` | worktree 被移除 | 無 matcher |
| `PreCompact` | context 壓縮前 | `manual`, `auto` |
| `SessionEnd` | session 結束 | `clear`, `logout`, `prompt_input_exit`, `bypass_permissions_disabled`, `other` |

### 4.3 Hook 類型

| 類型 | 說明 |
|------|------|
| `command` | 執行 shell 指令，透過 stdin 接收 JSON，透過 stdout/stderr/exit code 回傳結果 |
| `http` | POST event data 到 HTTP endpoint |
| `prompt` | 單次 LLM 呼叫，用 Claude 模型（預設 Haiku）做判斷 |
| `agent` | 多輪 agent 驗證，可使用工具讀檔、搜尋等 |

### 4.4 Hook 設定格式

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "jq -r '.tool_input.file_path' | xargs npx prettier --write"
          }
        ]
      }
    ]
  }
}
```

### 4.5 Matcher 語法

Matcher 是 **regex 模式**，匹配特定的欄位：

```json
{"matcher": "Edit|Write"}           // 匹配 Edit 或 Write
{"matcher": "Bash"}                 // 僅匹配 Bash
{"matcher": "mcp__github__.*"}      // 所有 github MCP 工具
{"matcher": ""}                     // 匹配所有（空字串 = 萬用）
{"matcher": "compact"}              // SessionStart 時僅匹配 compact 觸發
```

### 4.6 Hook 輸入（stdin JSON）

每個事件都包含共用欄位，加上事件特定的資料：

```json
{
  "session_id": "abc123",
  "cwd": "/Users/sarah/myproject",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {
    "command": "npm test"
  }
}
```

### 4.7 Hook 輸出與 Exit Code

| Exit Code | 效果 |
|-----------|------|
| `0` | 動作繼續。stdout 內容加入 Claude 的 context |
| `2` | 動作**被阻擋**。stderr 內容作為回饋送給 Claude |
| 其他 | 動作繼續。stderr 記錄但不顯示給 Claude |

**進階：結構化 JSON 輸出**（exit 0 + JSON stdout）：

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "請用 rg 而不是 grep 以獲得更好的效能"
  }
}
```

`PreToolUse` 的 `permissionDecision` 可為：`"allow"`、`"deny"`、`"ask"`。

### 4.8 Hook 設定位置

| 位置 | 範圍 | 可共享 |
|------|------|--------|
| `~/.claude/settings.json` | 所有專案 | 否 |
| `.claude/settings.json` | 單一專案 | 是，可提交到 repo |
| `.claude/settings.local.json` | 單一專案 | 否，gitignored |
| Managed policy settings | 組織全域 | 是，管理者控制 |
| Plugin `hooks/hooks.json` | Plugin 啟用時 | 是，隨 plugin 打包 |
| Skill/Agent frontmatter | skill/agent 啟動時 | 是 |

### 4.9 實用 Hook 範例

#### 桌面通知（Claude 需要輸入時）

```json
{
  "hooks": {
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "notify-send 'Claude Code' 'Claude Code needs your attention'"
          }
        ]
      }
    ]
  }
}
```

#### 自動格式化（編輯檔案後）

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "jq -r '.tool_input.file_path' | xargs npx prettier --write"
          }
        ]
      }
    ]
  }
}
```

#### 保護敏感檔案（阻止編輯）

```bash
#!/bin/bash
# .claude/hooks/protect-files.sh
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
PROTECTED_PATTERNS=(".env" "package-lock.json" ".git/")

for pattern in "${PROTECTED_PATTERNS[@]}"; do
  if [[ "$FILE_PATH" == *"$pattern"* ]]; then
    echo "Blocked: $FILE_PATH matches protected pattern '$pattern'" >&2
    exit 2
  fi
done
exit 0
```

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "\"$CLAUDE_PROJECT_DIR\"/.claude/hooks/protect-files.sh"
          }
        ]
      }
    ]
  }
}
```

#### Context 壓縮後重新注入資訊

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "echo 'Reminder: use Bun, not npm. Run bun test before committing.'"
          }
        ]
      }
    ]
  }
}
```

#### Prompt-based Hook（用 LLM 判斷是否完成）

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "prompt",
            "prompt": "Check if all tasks are complete. If not, respond with {\"ok\": false, \"reason\": \"what remains to be done\"}."
          }
        ]
      }
    ]
  }
}
```

#### Agent-based Hook（執行驗證後再停止）

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "agent",
            "prompt": "Verify that all unit tests pass. Run the test suite and check the results. $ARGUMENTS",
            "timeout": 120
          }
        ]
      }
    ]
  }
}
```

#### HTTP Hook（POST event data 到外部服務）

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "hooks": [
          {
            "type": "http",
            "url": "http://localhost:8080/hooks/tool-use",
            "headers": {
              "Authorization": "Bearer $MY_TOKEN"
            },
            "allowedEnvVars": ["MY_TOKEN"]
          }
        ]
      }
    ]
  }
}
```

#### 設定變更審計

```json
{
  "hooks": {
    "ConfigChange": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "jq -c '{timestamp: now | todate, source: .source, file: .file_path}' >> ~/claude-config-audit.log"
          }
        ]
      }
    ]
  }
}
```

### 4.10 Hook 注意事項

- Hook 預設 timeout 為 **10 分鐘**，可用 `timeout` 欄位調整（單位：秒）
- `PostToolUse` hook **無法撤銷**已執行的動作
- `PermissionRequest` hook 在非互動模式（`-p`）**不會觸發**，用 `PreToolUse` 代替
- `Stop` hook 在 Claude 每次完成回應時觸發，不僅是任務完成時
- Stop hook 要防止無限迴圈，檢查 `stop_hook_active` 欄位

---

## 5. Marketplace 與上架流程

### 5.1 Marketplace 是什麼？

Marketplace 是 plugin 的目錄，讓使用者可以發現並安裝你的 plugin。運作方式是兩步驟：

1. **加入 marketplace**：註冊目錄到 Claude Code
2. **安裝個別 plugin**：從目錄中選擇要安裝的

### 5.2 marketplace.json Schema

放在 `.claude-plugin/marketplace.json`：

```json
{
  "name": "company-tools",                // 必要。kebab-case，使用者安裝時會看到
  "owner": {                              // 必要
    "name": "DevTools Team",
    "email": "devtools@example.com"       // 選填
  },
  "metadata": {                           // 選填
    "description": "公司內部工具集",
    "version": "1.0.0",
    "pluginRoot": "./plugins"             // plugin 相對路徑的基礎目錄
  },
  "plugins": [
    {
      "name": "code-formatter",           // 必要
      "source": "./plugins/formatter",    // 必要，來源（見下方）
      "description": "自動程式碼格式化",
      "version": "2.1.0",
      "author": { "name": "DevTools Team" },
      "homepage": "https://docs.example.com",
      "repository": "https://github.com/...",
      "license": "MIT",
      "keywords": ["formatter"],
      "category": "productivity",
      "tags": ["code-quality"],
      "strict": true                      // 預設 true
    }
  ]
}
```

### 5.3 Plugin Source 類型

| Source 類型 | 格式 | 說明 |
|------------|------|------|
| 相對路徑 | `"./plugins/my-plugin"` | 同 repo 內的目錄，必須以 `./` 開頭 |
| GitHub | `{"source": "github", "repo": "owner/repo", "ref?": "v2.0", "sha?": "abc..."}` | GitHub 倉庫 |
| Git URL | `{"source": "url", "url": "https://gitlab.com/team/plugin.git", "ref?": "...", "sha?": "..."}` | 任何 Git 倉庫 |
| Git 子目錄 | `{"source": "git-subdir", "url": "...", "path": "tools/plugin", "ref?": "...", "sha?": "..."}` | monorepo 中的子目錄，sparse clone |
| npm | `{"source": "npm", "package": "@acme/plugin", "version?": "^2.0.0", "registry?": "..."}` | npm 套件 |
| pip | `{"source": "pip", "package": "my-plugin", "version?": "...", "registry?": "..."}` | pip 套件 |

### 5.4 上架到官方 Marketplace

官方 marketplace（`claude-plugins-official`）內建於 Claude Code。提交 plugin：

- **Claude.ai**：[claude.ai/settings/plugins/submit](https://claude.ai/settings/plugins/submit)
- **Console**：[platform.claude.com/plugins/submit](https://platform.claude.com/plugins/submit)

### 5.5 建立自己的 Marketplace

**步驟**：

1. 建立 repo 結構：

```
my-marketplace/
├── .claude-plugin/
│   └── marketplace.json
└── plugins/
    └── my-plugin/
        ├── .claude-plugin/
        │   └── plugin.json
        └── skills/
            └── review/
                └── SKILL.md
```

2. Push 到 GitHub

3. 使用者安裝：

```bash
# 加入 marketplace
/plugin marketplace add your-org/my-marketplace

# 安裝 plugin
/plugin install my-plugin@your-org-my-marketplace
```

### 5.6 Marketplace 來源類型

使用者可以從以下來源加入 marketplace：

```bash
# GitHub repo
/plugin marketplace add owner/repo

# 任何 Git host
/plugin marketplace add https://gitlab.com/company/plugins.git

# 特定 branch/tag
/plugin marketplace add https://gitlab.com/company/plugins.git#v1.0.0

# 本地目錄
/plugin marketplace add ./my-marketplace

# 遠端 URL
/plugin marketplace add https://example.com/marketplace.json
```

### 5.7 Marketplace 管理指令

```bash
/plugin marketplace list           # 列出所有 marketplace
/plugin marketplace update <name>  # 更新 marketplace
/plugin marketplace remove <name>  # 移除 marketplace（會一併移除從此安裝的 plugin）
```

縮寫：`/plugin market` = `/plugin marketplace`，`rm` = `remove`

### 5.8 自動更新

- 官方 marketplace 預設啟用自動更新
- 第三方 marketplace 預設停用
- 在 `/plugin` → Marketplaces → 選擇 marketplace → Enable/Disable auto-update
- 環境變數：`DISABLE_AUTOUPDATER` 停用所有更新，`FORCE_AUTOUPDATE_PLUGINS=true` 僅保留 plugin 更新

### 5.9 Strict Mode

`strict` 欄位控制 `plugin.json` vs `marketplace.json` 的權威性：

| 值 | 行為 |
|-----|------|
| `true`（預設） | `plugin.json` 是主體，marketplace entry 補充 |
| `false` | marketplace entry 是完整定義，plugin 不需要自己的 `plugin.json` |

---

## 6. 本地開發與測試

### 6.1 用 `--plugin-dir` 測試

```bash
# 載入單一 plugin
claude --plugin-dir ./my-plugin

# 載入多個 plugin
claude --plugin-dir ./plugin-one --plugin-dir ./plugin-two
```

- 改檔案後執行 `/reload-plugins` 即可熱更新
- LSP server 變更仍需重啟

### 6.2 本地 Marketplace 測試

```bash
# 加入本地 marketplace
/plugin marketplace add ./my-marketplace

# 安裝測試
/plugin install test-plugin@my-marketplace
```

### 6.3 驗證 Plugin

```bash
claude plugin validate .
# 或在 session 中
/plugin validate .
```

### 6.4 Debug

```bash
# 完整 debug 輸出
claude --debug

# 在 session 中
/debug
```

在 session 中按 `Ctrl+O` 可切換 verbose 模式，看到 hook 輸出。

---

## 7. 團隊部署與企業管理

### 7.1 團隊 Marketplace 設定

在 `.claude/settings.json` 中配置：

```json
{
  "extraKnownMarketplaces": {
    "company-tools": {
      "source": {
        "source": "github",
        "repo": "your-org/claude-plugins"
      }
    }
  },
  "enabledPlugins": {
    "code-formatter@company-tools": true,
    "deployment-tools@company-tools": true
  }
}
```

團隊成員信任 repo 資料夾後，Claude Code 會提示安裝。

### 7.2 Managed Marketplace 限制

管理者可透過 `strictKnownMarketplaces` 限制可加入的 marketplace：

```json
{
  "strictKnownMarketplaces": []               // 完全鎖定，不能加任何 marketplace
}
```

```json
{
  "strictKnownMarketplaces": [                // 僅允許特定 marketplace
    { "source": "github", "repo": "acme-corp/approved-plugins" },
    { "source": "hostPattern", "hostPattern": "^github\\.example\\.com$" }
  ]
}
```

### 7.3 權限控制

```json
{
  "permissions": {
    "allow": [
      "Bash(xreview *)",
      "Read(/src/**)",
      "Skill(commit)",
      "Skill(review-pr *)"
    ],
    "deny": [
      "Bash(rm -rf *)",
      "Edit(.env)",
      "Skill(deploy *)"
    ]
  }
}
```

**優先級**：deny > ask > allow。

---

## 8. 參考資料

### 官方文件

| 文件 | URL |
|------|-----|
| Plugin 建立 | https://code.claude.com/docs/en/plugins |
| Plugin 技術參考 | https://code.claude.com/docs/en/plugins-reference |
| 發現與安裝 Plugin | https://code.claude.com/docs/en/discover-plugins |
| Plugin Marketplace | https://code.claude.com/docs/en/plugin-marketplaces |
| Skill 指南 | https://code.claude.com/docs/en/skills |
| Hook 指南 | https://code.claude.com/docs/en/hooks-guide |
| Hook 技術參考 | https://code.claude.com/docs/en/hooks |
| Subagent | https://code.claude.com/docs/en/sub-agents |
| 權限設定 | https://code.claude.com/docs/en/permissions |
| 設定檔 | https://code.claude.com/docs/en/settings |

### 版本要求

- Claude Code **1.0.33** 或更新版本
- 確認版本：`claude --version`
- 升級：`brew upgrade claude-code`（Homebrew）或 `npm update -g @anthropic-ai/claude-code`（npm）

### Agent Skills 開放標準

- https://agentskills.io
