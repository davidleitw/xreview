# xreview

Agent-native 程式碼審查引擎，支援 Claude Code 與 Codex CLI，由 Codex 驅動。

xreview 把程式碼審查委託給 Codex（另一個 AI 模型），讓你的 coding agent 獲得獨立的第二意見。它協調三方審查迴圈：**Codex 審查、你的 agent 驗證、你做決定。**

**[English README](../README.md)**

## 運作方式

當你請你的 coding agent 審查程式碼時，xreview skill 會自動接手：

1. **Codex 審查**你的程式碼，回報發現的問題（bug、安全漏洞、邏輯錯誤）
2. **你的 agent 獨立驗證**每個問題 — 實際讀取原始碼，確認或挑戰可能的誤報，透過與 Codex 討論來過濾 false positive
3. **你的 agent 呈現**修復計畫（僅包含已驗證的問題）— 觸發條件、影響、連鎖影響和修復方案
4. **你來決定** — 全部按推薦修、只修高嚴重度、或逐條調整
5. **你的 agent 修正**嚴格按你批准的計畫執行
6. **Codex 驗證**修正結果，可能發現新問題或重新開啟被駁回的項目
7. **重複**直到三方達成共識（最多 5 輪）
8. **總結** — 你的 agent 在對話中產生詳細的口頭總結，涵蓋所有問題、決策和修復內容

這不是你的 agent 自己審查自己的程式碼，而是由不同模型提供真正獨立的審查，你的 agent 作為驗證層過濾誤報後再呈現給你。

## 安裝

### Claude Code

註冊 marketplace 並安裝：

```bash
/plugin marketplace add davidleitw/xreview
/plugin install xreview@xreview-marketplace
```

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

### 前置需求

- 安裝並認證 [Codex CLI](https://github.com/openai/codex)（`npm install -g @openai/codex`）
- 設定好 Codex 的 OpenAI API key

## 使用方式

直接請你的 coding agent 審查：

```
用 xreview 檢查這段程式碼有沒有 bug 和安全問題
```

或指定檔案：

```
用 xreview review store/db.go 和 handler/exec.go，檢查安全漏洞
```

xreview skill 會自動觸發。在 Claude Code 中也可以直接用 `/xreview` 呼叫。

### 可以抓到什麼

| 類別 | 範例 |
|------|------|
| **安全性** | SQL injection、command injection、硬編碼密鑰、缺少認證 |
| **邏輯** | Nil pointer、race condition、off-by-one |
| **錯誤處理** | 忽略 error、resource leak、未關閉連線 |
| **效能** | N+1 query、不必要的記憶體分配 |

### 語言特化審查

xreview 透過 `--language` 支援語言感知的審查。當 skill 偵測到審查目標是支援的語言時，會自動將語言特定的審查規則加入 Codex prompt。

| 語言 | Key | 審查規則 |
|------|-----|----------|
| C++ | `cpp` | ISO C++ Core Guidelines — 記憶體安全、UB、並行、例外安全、所有權、類別設計 |
| Go | `go` | Effective Go + Go Code Review Comments — goroutine 安全、data race、資源洩漏、錯誤處理、並行模式 |

不支援的語言會回退到通用審查模式（與不帶 flag 的行為相同）。

### 三方審查迴圈

每個問題都會經過結構化分析：

```
F-001: SQL Injection (security/high)
  store/db.go:34 — FindUser()

觸發條件：使用者透過 /user?name=' OR '1'='1 送入惡意字串
根本原因：fmt.Sprintf 直接將使用者輸入拼接進 SQL 查詢
影響：攻擊者可以讀取、修改或刪除資料庫中任意資料

-> 修正：改用參數化查詢 db.Query("...WHERE name = ?", name)
```

- **所有問題一次呈現** — 你在任何程式碼修改之前看到全貌
- **每個問題列出多個修復方案** — 標明工作量和推薦，你來選
- **每個問題都可以選「不修」** — 最終決定權永遠在你

修完後 Codex 會驗證。如果它不同意某個駁回或覺得修得不完整，迴圈會繼續。

## 自動更新

xreview 會自動保持最新版本。每次 review 的 preflight 階段會檢查 GitHub Releases 是否有新版本，檢查結果會在本地快取 24 小時，不會拖慢流程。

發現新版本時，skill 會在繼續 review 之前自動執行 `xreview self-update`。更新方式是直接從 GitHub Releases 下載對應你作業系統和架構的預編譯 binary，不需要 Go 工具鏈。如果更新失敗，xreview 會繼續使用當前版本，不影響 review。

也可以手動更新：

```bash
xreview self-update
```

## CLI 參考

xreview 是一個獨立的 Go binary，你的 coding agent 在背後呼叫它：

| 指令 | 用途 |
|------|------|
| `xreview preflight` | 檢查環境（codex 安裝、API key、版本、更新） |
| `xreview review --files <paths>` | 執行初始審查 |
| `xreview review --files <paths> --language <key>` | 使用語言特化規則審查（cpp, go） |
| `xreview review --session <id> --message "..."` | 恢復驗證輪次 |
| `xreview clean --session <id>` | 清理單一 session |
| `xreview clean --all` | 清理所有 session |
| `xreview self-update` | 從 GitHub Releases 更新到最新版本 |
| `xreview version` | 顯示版本 |

## 開發

```bash
git clone https://github.com/davidleitw/xreview.git
cd xreview
go build -o xreview ./cmd/xreview/
```

在 Claude Code 中載入本地 plugin（不需要從 marketplace 安裝）：

```bash
claude --plugin-dir .
```

這會透過 `.claude-plugin/plugin.json` 載入 `skills/` 目錄的 skill。編輯 skill 檔案後，在 session 中執行 `/reload-plugins` 即可熱載入。

## 架構

```
Host Agent                  xreview (CLI)           Codex (reviewer)
(Claude Code / Codex CLI)
     |                          |                        |
     |-- review request ------->|                        |
     |                          |-- codex exec --------->|
     |                          |   (Codex 自行讀取程式碼  |
     |                          |    透過 git diff/檔案)   |
     |                          |<-- findings (JSON) ----|
     |                          |  [快照檔案 checksum]     |
     |<-- findings (XML) ------|                        |
     |                          |                        |
     |  [逐一驗證每個問題]        |                        |
     |  [挑戰可疑項目] --------->|-- codex resume ------->|
     |                          |<-- 重新評估 ------------|
     |                          |                        |
     |  [呈現修復計畫]            |                        |
     |  [使用者批准]              |                        |
     |  [修正程式碼]              |                        |
     |                          |                        |
     |-- resume --------------->|  [比對 checksum         |
     |                          |   偵測變動檔案]          |
     |                          |-- codex resume ------->|
     |                          |   (prompt 包含           |
     |                          |    變動檔案清單)          |
     |                          |<-- verify (JSON) ------|
     |<-- verify (XML) --------|                        |
     |                          |                        |
     |  [口頭總結]               |                        |
     |-- clean ---------------->|                        |
```

- xreview 在 stdout 輸出 XML 供 skill 消費
- Codex 自行取得程式碼（以唯讀模式執行 `git diff` 或讀取檔案）
- 你的 coding agent 獨立驗證每個問題後才呈現給使用者
- Session 狀態以 JSON 儲存在 `/tmp/xreview/sessions/`（暫時性）
- 多輪審查：透過 `--session <session-id>` 恢復 codex session
- 檔案快照（SHA-256 checksum）追蹤每輪之間的變動 — xreview 偵測哪些檔案有修改並通知 Codex 重新讀取，確保審查永遠是基於最新程式碼

## 未來方向

- **第二意見 (Second Opinion)** — 將同一份程式碼送給第二個獨立的 reviewer（不同模型或不同的 prompt 焦點），彙整發現。每個 reviewer 有自己的 session；xreview 在呈現給使用者前合併並去重。
- **審查計畫 (Review Plan)** — 單輪、唯讀的審查模式，產出結構化的審查計畫（要檢查什麼、檢查順序、要注意哪些 pattern），而不實際執行審查。適用於大型 codebase，在投入完整 review 之前先界定範圍。
- **更多語言特化規則** — `--language` 目前支援 C++ 和 Go。計畫新增 Rust、TypeScript、Python 等語言。
- **全自動修復模式 (`--auto-fix`)** — 針對 vibe coding 工作流的全自動 review + 修復循環。跳過 review-only 討論階段，自動套用建議修復並走三方驗證 loop，全程不需要用戶介入。

## 移除

### Claude Code

```
/plugin uninstall xreview
```

### Codex CLI

```bash
rm -rf ~/.agents/skills/xreview
```

### 清除 binary 和快取資料

```bash
# 移除 binary（確認你的安裝路徑）
rm "$(which xreview)"

# 移除版本快取
rm -rf ~/.cache/xreview

# 移除 session 資料（可選，存放在 /tmp）
rm -rf /tmp/xreview
```

## 授權

MIT License — 詳見 [LICENSE](../LICENSE)。

## 支援

- **Issues**: https://github.com/davidleitw/xreview/issues
