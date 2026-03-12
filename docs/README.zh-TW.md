# xreview

Agent-native 程式碼審查引擎，專為 Claude Code 設計，由 Codex 驅動。

xreview 把程式碼審查委託給 Codex（另一個 AI 模型），讓 Claude Code 獲得獨立的第二意見。它協調三方審查迴圈：**Codex 審查、Claude Code 修正、你做決定。**

**[English README](../README.md)**

## 運作方式

當你請 Claude Code 審查程式碼時，xreview skill 會自動接手：

1. **Codex 審查**你的程式碼，回報發現的問題（bug、安全漏洞、邏輯錯誤）
2. **Claude Code 獨立驗證**每個問題 — 實際讀取原始碼，確認或挑戰可能的誤報，透過與 Codex 討論來過濾 false positive
3. **Claude Code 呈現**修復計畫（僅包含已驗證的問題）— 觸發條件、影響、連鎖影響和修復方案
4. **你來決定** — 全部按推薦修、只修高嚴重度、或逐條調整
5. **Claude Code 修正**嚴格按你批准的計畫執行
6. **Codex 驗證**修正結果，可能發現新問題或重新開啟被駁回的項目
7. **重複**直到三方達成共識（最多 5 輪）
8. **產生報告** — 人類可讀的 markdown 報告，總結問題、決策和修復內容

這不是 Claude Code 自己審查自己的程式碼，而是由不同模型提供真正獨立的審查，Claude Code 作為驗證層過濾誤報後再呈現給你。

## 安裝

### Claude Code

註冊 marketplace 並安裝：

```bash
/plugin marketplace add davidleitw/xreview
/plugin install xreview@xreview-marketplace
```

### 前置需求

- 安裝並認證 [Codex CLI](https://github.com/openai/codex)（`npm install -g @openai/codex`）
- 設定好 Codex 的 OpenAI API key

## 使用方式

直接請 Claude Code 審查程式碼：

```
幫我 review 這段程式碼
```

或指定檔案：

```
Review store/db.go 和 handler/exec.go，檢查安全漏洞
```

xreview skill 會自動觸發。也可以直接呼叫：

```
/xreview
```

### 可以抓到什麼

| 類別 | 範例 |
|------|------|
| **安全性** | SQL injection、command injection、硬編碼密鑰、缺少認證 |
| **邏輯** | Nil pointer、race condition、off-by-one |
| **錯誤處理** | 忽略 error、resource leak、未關閉連線 |
| **效能** | N+1 query、不必要的記憶體分配 |

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

xreview 是一個獨立的 Go binary，Claude Code 在背後呼叫它：

| 指令 | 用途 |
|------|------|
| `xreview preflight` | 檢查環境（codex 安裝、API key、版本、更新） |
| `xreview review --files <paths>` | 執行初始審查 |
| `xreview review --session <id> --message "..."` | 恢復驗證輪次 |
| `xreview report --session <id>` | 產生最終報告 |
| `xreview clean --session <id>` | 清理 session 資料 |
| `xreview self-update` | 從 GitHub Releases 更新到最新版本 |
| `xreview version` | 顯示版本 |

## 從源碼建構

```bash
git clone https://github.com/davidleitw/xreview.git
cd xreview
go build -o xreview ./cmd/xreview/
```

## 架構

```
Claude Code (host)          xreview (CLI)           Codex (reviewer)
     |                          |                        |
     |-- /xreview skill ------->|                        |
     |                          |-- codex exec --------->|
     |                          |   (Codex 自行讀取程式碼  |
     |                          |    透過 git diff/檔案)   |
     |                          |<-- findings (JSON) ----|
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
     |-- resume --------------->|                        |
     |                          |-- codex resume ------->|
     |                          |<-- verify (JSON) ------|
     |<-- verify (XML) --------|                        |
     |                          |                        |
     |  [write-report skill]    |                        |
     |-- report --------------->|                        |
     |<-- session 資料 ---------|                        |
     |  [產生 markdown 報告]     |                        |
```

- xreview 在 stdout 輸出 XML 供 Claude Code skill 消費
- Codex 自行取得程式碼（以唯讀模式執行 `git diff` 或讀取檔案）
- Claude Code 獨立驗證每個問題後才呈現給使用者
- 內部狀態以 JSON 儲存在 `.xreview/sessions/`
- 多輪審查：透過 `--resume <session-id>` 恢復 codex session
- 人類可讀的 markdown 報告由 write-report skill 產生

## 未來方向

- **語言感知的審查上下文** — 自動偵測專案主要開發語言，將該語言的 best practice（例如 Go 的 error handling 慣例、Rust 的 ownership 規則、Python 的型別安全）作為額外 context 傳給 Codex，讓審查結果更貼合該語言的慣用寫法和規範。

## 移除

從 Claude Code 移除 plugin：

```
/plugin uninstall xreview
```

然後清除 binary 和快取資料：

```bash
# 移除 binary（確認你的安裝路徑）
rm "$(which xreview)"

# 移除版本快取
rm -rf ~/.cache/xreview

# 移除 session 資料（可選）
# 每次 review 會在專案根目錄建立 .xreview/ 資料夾。
# 正常流程結束時 xreview 會詢問是否清除。
# 如果當時跳過了，手動刪除即可：
rm -rf /path/to/your/project/.xreview
```

## 授權

MIT License — 詳見 [LICENSE](../LICENSE)。

## 支援

- **Issues**: https://github.com/davidleitw/xreview/issues
