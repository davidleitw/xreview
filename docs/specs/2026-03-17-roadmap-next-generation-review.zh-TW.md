# xreview 下一代 Review — 路線圖與設計

日期: 2026-03-17
狀態: 草稿

**[English Version](2026-03-17-roadmap-next-generation-review.md)**

## 摘要

xreview 的核心價值是 **跨模型 code review**：Codex 找問題、Claude Code 獨立驗證、用戶做決策。這個三方模型消除了單一模型 review 工具的視覺盲區（包括 Anthropic 官方的 multi-agent code-review plugin——它跑 4 個 Sonnet instance，但共享完全相同的訓練偏差）。

本文件記錄 xreview 下一代功能的策略路線圖，基於：
- 生產環境 review session 的實戰回饋
- CodeRabbit、Greptile、Augment、Sourcery、Qodo 及 Anthropic 官方 plugin 的競品分析
- 如何利用 Claude Code 作為智慧型 orchestrator 的架構討論

---

## 問題：xreview 目前抓不到什麼

### 能抓到什麼 vs. 抓不到什麼

xreview 能有效抓到 **code pattern 問題**——在單一函數或檔案內可見的問題：缺少錯誤處理、交易安全漏洞、schema migration 問題。這些是「code vs. 正確性」的問題。

但它持續漏掉 **語義落差問題（semantic gap）**——code 運作正確，但無法傳達開發者的設計意圖。這類問題需要理解 code 背後的設計：資料結構如何跨檔案關聯、函數名暗示的行為 vs. 它 return 之後實際發生的事、一個常數的命名是否在每個使用點都符合其語義角色。

這個差距是根本性的：Codex review code 的方式是分析它 **做了什麼**。語義落差問題是關於 code **應該傳達什麼**——完全不同類型的 review。要抓到它們，需要跨檔案的結構性理解（symbol 使用模式、call chain 追蹤、資料結構形狀感知），這是單函數分析無法提供的。

### 雞生蛋蛋生雞問題

`--context` flag 理論上能幫忙——如果 reviewer 有發現語義不匹配所需的架構 context，它就能抓到。但寫出 misleading code 的開發者通常 **不知道自己在這樣做**。要求他們提供能揭露問題的 context，本身就是循環論證。

---

## 競品分析

### 各工具的做法

| 工具 | 架構 | 跨檔分析 | Context 策略 |
|---|---|---|---|
| **CodeRabbit** | Pipeline + agentic 混合 | Dependency graph、multi-repo | Vector DB (LanceDB)、1:1 code-to-context ratio、歷史 review 學習 |
| **Greptile** | 全 agent (Claude Agent SDK) | Multi-hop 調查——遞迴追蹤 call chain | 整個 codebase 的語義索引 |
| **Augment** | Context Engine | 40 萬+ 檔案語義索引、完整 dependency graph | 跨檔關係的語義搜尋 |
| **Sourcery** | Multi-angle 專家 | 每個 reviewer 各自的範圍 | 規則引擎 + LLM 混合 |
| **Anthropic plugin** | 4 個平行 Sonnet agent | Git blame 分析 | CLAUDE.md、confidence scoring (閾值 80) |
| **Qodo/PR-Agent** | 每個工具一個 LLM | PR 範圍 | PR 歷史 + codebase context |

### xreview 的差異化

1. **跨模型多樣性** — Codex 和 Claude 有不同的訓練資料、不同的 attention pattern、不同的盲區。這不是 bug，是核心功能。同模型 multi-agent（Anthropic 的 4 個 Sonnet）無法複製這一點。

2. **Claude Code 作為智慧驗證者** — 不只是 pass-through。Claude Code 獨立讀 code、確認或挑戰 findings、過濾 false positives。沒有其他工具有這麼強的驗證層。

3. **本地優先、免建索引** — 不需要 vector DB、dependency graph 基礎設施、SaaS 依賴。用 Claude Code 的即時讀取 + Codex 的 sandbox 存取。安裝 Codex CLI 就能用。

4. **用戶在迴圈中** — Review-only 模式讓用戶討論 findings、挑戰它們、選擇性觸發修復。不是在 PR 上倒一堆 comment 的 CI bot。

---

## 策略方向

### 核心命題

> **不要造更好的 Codex。造更好的 Codex orchestrator。**

Claude Code 在讀懂和理解 code 方面極為強大。差距不在 Codex 的 review 能力——而在我們給它的 **context 和策略**。下一代 xreview 應該：

1. 讓 Claude Code 在 review 前 **收集結構性 context**（這是機械性工作，不是 review 判斷）
2. 讓 Claude Code 根據看到的 code 來 **設計 review 策略**
3. 讓 xreview **派發帶有豐富 context 的 focused Codex review**
4. 讓 Claude Code **交叉驗證並整合** 結果

### 架構演進

**目前 (v0.8):**
```
User → Skill → xreview review → 1 個 Codex exec → findings → Claude Code 驗證 → User
```

**下一代：**
```
User → Skill（教 Claude Code 如何思考 review）
  │
  ├── Claude Code：判斷 scope（用戶指令 / commit / 記憶）
  ├── Claude Code：讀 code，建立結構性 context
  ├── Claude Code：決定 review 策略（單一 vs multi-angle）
  │
  ├── [單一] xreview review --files ... --context-file ctx.json
  │
  ├── [多角度] xreview review --files ... --focus "角度 1" --context-file ctx.json --session-group grp &
  │             xreview review --files ... --focus "角度 2" --context-file ctx.json --session-group grp &
  │             xreview review --files ... --focus "角度 3" --context-file ctx.json --session-group grp &
  │             wait
  │             xreview merge --session-group grp
  │
  ├── Claude Code：驗證 + 交叉驗證 findings
  └── User：討論、決策、修復
```

**關鍵轉變：** Skill 變得更厚——描述概念性的 review 方法論，而不只是 CLI 呼叫步驟。Claude Code 做策略決策，xreview CLI 保持是 focused toolkit。

---

## 路線圖

### Phase 1：Context Engineering（基礎建設）

**目標：** 給 Codex 抓住語義問題所需的結構性 context，不用建索引或寫 parser。

#### 1a. `--context-file <path>` flag

用檔案替換有長度限制的 `--context` 字串。Claude Code 在呼叫 xreview 前寫好結構化 JSON/YAML；xreview 讀取後注入 Codex prompt 的 context 區。

```bash
xreview review --files handler.go,repo.go --context-file .xreview/context.json
```

Context 檔案結構（由 Claude Code 準備）：

```json
{
  "project_summary": "HTTP API，分層架構：Handler → Service → Repository → PostgreSQL",

  "key_data_structures": {
    "OrderCache": "map<string, []Order> — per-customer order lists",
    "GlobalOrderList": "[]Order — flat list across all customers"
  },

  "symbol_cross_references": {
    "kMaxCacheSize": [
      {"file": "cache.go:15", "usage": "定義, value=1000"},
      {"file": "handler.go:42", "usage": "len(globalList) < k — flat list, per-system 語義"},
      {"file": "handler.go:67", "usage": "len(customerOrders) < k — per-customer list, per-element 語義"},
      {"file": "repo.go:120", "usage": "SQL LIMIT clause — per-query 語義"}
    ]
  },

  "lifecycles": [
    "Order: create → validate → enqueue → process → ship → confirm → archive"
  ],

  "review_hints": [
    "OrderCache 用 per-customer lists（map of slices），不同於 GlobalOrderList 的 flat list",
    "markProcessed() 只從 work queue dequeue，order 繼續走 shipping pipeline"
  ]
}
```

**為什麼由 Claude Code 準備，而非用戶：** Claude Code 可以機械性地 grep symbol 使用點、追蹤 call chain、讀 struct 定義。它不需要判斷什麼是錯的——只是萃取結構性事實。雞生蛋問題被避免了，因為這是觀察，不是診斷。

**CLI 改動：**
- 新 flag：`--context-file <path>` — 讀檔案，注入 prompt context 區
- `--context` 保留（向後相容，短 inline context）
- 兩者可同時使用；context-file 內容排在前面

#### 1b. `--focus <string>` flag

告訴 Codex 這次 review 專注在什麼角度。注入到 prompt 的 instruction 區（不是 context）。

```bash
xreview review --files handler.go,cache.go --focus "跨檔案的常數和 enum 語義一致性"
```

**區分：**
- `--context-file` = **reviewer 應該知道的背景知識**
- `--focus` = **reviewer 應該找的方向**（review 指令）

#### 1c. `--git-diff <ref>` scope 模式

Review refs 之間的變更，不只是未提交的或明確指定的檔案。

```bash
xreview review --git-diff HEAD~3          # 最近 3 個 commit
xreview review --git-diff origin/main     # 從 main 分支以來的變更
xreview review --git-diff abc123..def456  # 特定範圍
```

xreview 執行 `git diff <ref> --name-only` 取得變更檔案，然後按 `--files` 流程處理。

#### 1d. Codex output schema 加入 Confidence scoring

在 finding schema 加入 `confidence` 欄位 (0-100)。Codex 根據對 finding 的確定程度給分。xreview 透傳；Claude Code 在驗證階段使用（低 confidence 的 findings 會被額外審視）。

```json
{
  "findings": [{
    "title": "batch update 缺少錯誤處理",
    "severity": "medium",
    "confidence": 85,
    "category": "error-handling",
    ...
  }]
}
```

不自動用閾值過濾——由 Claude Code 在驗證階段決定怎麼處理 confidence 分數。

### Phase 2：Multi-Angle Review

**目標：** 讓 Claude Code 平行派發多個 focused Codex review 並合併結果。

**依賴：** Phase 1（context-file 和 focus flags）。

#### 2a. `--session-group <group-id>` flag

將 review session 標記為某個 group 的一部分。同一 group ID 的多個 `xreview review` 呼叫邏輯上關聯。

```bash
GROUP="grp-$(date +%s)"

# Claude Code 用 Agent tool 平行派發
xreview review --files handler.go,cache.go \
  --focus "跨檔案常數/enum 語義一致性" \
  --context-file ctx.json --session-group $GROUP

xreview review --files handler.go,service.go,repo.go \
  --focus "函數命名 vs 跨 request lifecycle 的實際行為" \
  --context-file ctx.json --session-group $GROUP

xreview review --files handler.go,repo.go \
  --focus "bugs、安全性、錯誤處理" \
  --context-file ctx.json --session-group $GROUP
```

每次呼叫照常在 `.xreview/sessions/` 建立自己的 session，同時在 `.xreview/groups/<group-id>.json` 註冊。

#### 2b. `xreview merge --session-group <group-id>`

合併一個 group 內所有 session 的 findings 為統一結果。

```bash
xreview merge --session-group $GROUP
```

合併邏輯：
1. 載入 group 內所有 session
2. 去重：指向同一 file:line 且描述相似的 findings → 保留 confidence 較高的，註記另一個角度也有標記
3. 統一編號：跨角度的 F-001, F-002, ...
4. 保留出處：每個 finding 記錄是哪個角度/session 發現的
5. 輸出：和單一 review 相同的 XML 格式，相容既有 skill 流程

**去重策略選項**（透過 `--strategy` 指定）：
- `dedup`（預設）：合併重疊的 findings，保留最高 confidence
- `union`：保留全部 findings，標記重複但不合併
- `intersect`：只保留被 2 個以上角度標記的 findings（高精度模式）

#### 2c. Skill 更新：review 策略決策

Skill 教 Claude Code 什麼時候用單一 vs multi-angle：

```markdown
## Review 策略

根據範圍和複雜度決定：

### 快速 Review（單一 Codex）
- 少量檔案（1-3 個），單一關注點
- 用戶要求特定檢查（「檢查 SQL injection」）
- 時間敏感，用戶要快速回饋

→ xreview review --files <paths> [--focus <specific>] [--context-file <ctx>]

### 深度 Review（multi-angle）
- 跨子系統的變更
- 複雜的資料流或 lifecycle
- 用戶要求徹底 review
- 你看到值得做跨切面分析的 code pattern

→ 準備 context-file、決定角度、平行派發 review、merge

### 如何決定角度
先讀 code。找：
- 跨多個檔案使用的常數/enum → 角度：「語義一致性」
- 多步驟 lifecycle（create → process → complete）→ 角度：「lifecycle 命名與狀態轉換」
- 複雜資料結構（map of lists、嵌套容器）→ 角度：「資料結構邊界檢查」
- 永遠包含一個通用角度：「bugs、安全性、錯誤處理」
```

#### 2d. Multi-angle 驗證回合

修復後，Claude Code 需要跨所有角度驗證。兩個選項：

- **選項 A：** 分別 resume 每個原始 session，再次 merge。保留每個角度的 Codex 對話 context。
- **選項 B：** 建立單一新 session 做驗證。較簡單但失去每個角度的 context。

建議：**選項 A** — 每個角度的對話 context 很有價值。Claude Code 平行派發 `xreview review --session <id> --message "..."` 到各角度的 session，然後再次 `xreview merge`。

### Phase 3：增強 Skill 智慧

**目標：** 在不改 CLI 的情況下讓 skill 的 review 方法論更聰明。

#### 3a. 結構性 context 收集指南

擴充 skill 來教 Claude Code 如何準備 context-file：

```markdown
## 準備 Context（呼叫 xreview 之前）

1. 讀目標檔案
2. 對每個 non-trivial symbol（常數、enum、關鍵函數）：
   - Grep 跨 codebase 的所有使用點
   - 記錄使用點之間的語義差異
3. 對命名暗示 lifecycle 的函數（done, complete, close, init, destroy）：
   - 追蹤函數 return 後發生什麼
   - 記錄是否還有重要工作未完成
4. 對關鍵資料結構：
   - 記錄它們的形狀（flat list? map of lists? tree?）
   - 記錄形狀在哪裡影響語義
5. 將結果寫入 context-file（結構化 JSON）
```

這是機械性工作——Claude Code 不是在 review，只是在觀察。

#### 3b. Review 記憶（跨 session 學習）

當用戶重複 dismiss 某類 finding，或出現 false positive pattern 時，儲存起來：

```json
// .xreview/config.json
{
  "learned_rules": [
    {
      "pattern": "defer Close() 缺少 error handling",
      "action": "suppress",
      "reason": "專案慣例：defer Close() 的 error 是刻意忽略的"
    }
  ]
}
```

xreview 將 learned rules 注入 Codex prompt。類似 CodeRabbit 的 learnings 和 Sourcery 的回饋適應機制。

#### 3c. 語言感知 review hints

從副檔名偵測專案語言，注入語言特定的 review pattern 到 prompt：

```
語言：C++
- 檢查 aggregate initialization 可讀性（return {}、designated initializers）
- 檢查 RAII 合規：constructor 取得的資源必須在 destructor 釋放
- 檢查隱式轉換和 narrowing

語言：Go
- 檢查所有 error 都有處理或有註解明確忽略
- 檢查 goroutine leak（無界的 goroutine spawning，沒有 lifecycle 管理）
- 檢查 context.Context 在 call chain 中的傳遞
```

這些是 prompt 層的增加，不是 code 改動。以資料檔形式存在 xreview 中，按語言選用。

### Phase 4：新 Review 模式

**目標：** 支援「有 code 變更的檔案」以外的 review 對象。

#### 4a. Design Plan Review (`xreview review-plan`)

在執行前 review implementation plan / design doc。不同的 prompt、不同的 schema、不同的 findings 結構。

```bash
xreview review-plan --file docs/specs/feature-x-design.md --codebase-context ctx.json
```

Codex 讀 plan + 相關的現有 code，檢查：
- 可行性問題（plan 假設了不存在的 API）
- plan 中缺少的錯誤處理
- 與現有 code 的架構衝突
- 不完整的 edge case 覆蓋
- 範圍問題（plan 想做太多）

Output schema 和 code review 不同——findings 是關於 plan 的，不是 code 位置。

#### 4b. Auto-Fix 模式 (`--auto-fix`)

全自動的 review-and-fix 循環。Claude Code 跳過 review-only 討論階段，自動透過三方驗證迴圈套用建議的修復。

```bash
# 在 skill 中，由用戶說「review 並修好所有問題」觸發
xreview review --files <paths> --context-file ctx.json
# Claude Code 自動修復所有 high/critical findings
# Resume 驗證
# 重複直到乾淨或達到最大回合數
```

這是 skill 層的改變（Claude Code 的行為），不是 CLI 改動。Skill 描述什麼時候適合 auto-fix（用戶明確要求、vibe coding 場景）、什麼時候不適合（production code、團隊 review 場景）。

### Phase 5：Multi-Model Review（未來）

**目標：** 支援 Codex 以外的 reviewer。

#### 5a. Reviewer 抽象

xreview 目前有 `internal/reviewer/reviewer.go` 介面和 `single.go`（Codex 實作）。擴展支援：

- **Gemini reviewer** — Google 的模型，和 Codex 有不同的盲區
- **Local model reviewer** — Ollama/llama.cpp，離線使用
- **Custom reviewer** — 用戶提供的命令，stdin 接收 prompt，stdout 輸出 JSON

```bash
xreview review --files <paths> --reviewer codex    # 預設
xreview review --files <paths> --reviewer gemini
xreview review --files <paths> --reviewer custom --reviewer-cmd "my-review-tool"
```

#### 5b. Second Opinion 模式

同一份 code 過多個 reviewer，各自有自己的 session：

```bash
xreview review --files <paths> --reviewers codex,gemini --session-group grp
# 內部 spawn 兩個 review 進程
# xreview merge 合併來自不同模型的 findings
```

跨模型 findings（被 Codex 和 Gemini 獨立標記的）提升 confidence。單一模型的 findings 仍呈現，但標記為 single-source。

---

## CLI 命令參考（完整）

### 現有（不變）

| 命令 | 用途 |
|---|---|
| `xreview preflight` | 檢查環境就緒度 |
| `xreview version` | 顯示版本 + 檢查更新 |
| `xreview self-update` | 更新到最新版 |
| `xreview report --session <id>` | 生成報告 |
| `xreview clean --session <id>` | 刪除 session 資料 |

### 強化

| 命令 | 改動 |
|---|---|
| `xreview review` | 新 flags：`--context-file`、`--focus`、`--git-diff`、`--session-group`、`--confidence-threshold` |

### 新增

| 命令 | 用途 | Phase |
|---|---|---|
| `xreview merge --session-group <id>` | 合併 multi-angle findings | 2 |
| `xreview report --session-group <id>` | 從合併結果生成報告 | 2 |
| `xreview clean --session-group <id>` | 清理整組 session | 2 |
| `xreview review-plan --file <path>` | Review design/implementation plan | 4 |

### `xreview review` 完整 flag 參考

```
xreview review [flags]

Scope（互斥）：
  --files <path,path,...>     指定檔案
  --git-uncommitted           所有未提交的變更
  --git-diff <ref>            相對於 git ref 的變更

Context：
  --context <string>          Inline context（短，向後相容）
  --context-file <path>       結構化 context 檔案（JSON/YAML）
  --focus <string>            Review 角度 / 要找的方向

Session：
  --session <id>              Resume 既有 session
  --session-group <group-id>  將 session 標記為 group 的一部分
  --message <string>          Session resume 時的訊息

Output：
  --confidence-threshold <n>  過濾低於 confidence 的 findings（預設: 0, 不過濾）
  --json                      輸出原始 JSON 而非 XML

Control：
  --timeout <seconds>         Codex 執行超時（預設: 120）
  --full-rescan               Resume 時強制重新讀取所有檔案
```

---

## 實作優先順序

| Phase | 工作量 | 影響 | 依賴 |
|---|---|---|---|
| **1a: --context-file** | 小 | 高 | 無——解鎖豐富 context |
| **1b: --focus** | 小 | 高 | 無——解鎖 multi-angle |
| **1c: --git-diff** | 小 | 中 | 無 |
| **1d: Confidence scoring** | 小 | 中 | Schema 改動 |
| **2a: --session-group** | 中 | 高 | 1a, 1b |
| **2b: xreview merge** | 中 | 高 | 2a |
| **2c: Skill 策略更新** | 中 | 高 | 2a, 2b |
| **2d: Multi-angle 驗證** | 中 | 中 | 2b |
| **3a: Context 收集指南** | 小（只改 skill） | 高 | 1a |
| **3b: Review 記憶** | 中 | 中 | — |
| **3c: 語言感知 hints** | 小 | 中 | — |
| **4a: review-plan** | 大 | 中 | 新 prompt + schema |
| **4b: Auto-fix 模式** | 中（只改 skill） | 中 | 穩定的 review loop |
| **5a: Reviewer 抽象** | 大 | 高 | 穩定的 merge |
| **5b: Second opinion** | 大 | 高 | 5a |

### 建議執行順序

```
Phase 1a + 1b（平行）→ 1c + 1d（平行）→ 3a（skill，可重疊）
  → Phase 2a → 2b → 2c + 2d（平行）
  → Phase 3b + 3c（平行，獨立）
  → Phase 4a 或 4b（依需求）
  → Phase 5（multi-angle 穩定後）
```

---

## 設計原則

1. **xreview 是工具箱，Claude Code 是大腦。** 不要在 CLI 裡放智慧。給 Claude Code 更好的工具和更好的指導（透過 skill），讓它做策略決策。

2. **跨模型多樣性是護城河。** 同模型 multi-agent review 有天花板。不同模型看到不同的東西。保護並擴大這個優勢。

3. **Context 勝過巧思。** 實際 review session 顯示，有正確 context 的 Codex 本來就能抓到那些語義問題。不要做靜態分析——做更好的 context pipeline。

4. **增量演進。** 每個 phase 都獨立有價值。光是 Phase 1（context-file + focus）就已經能改善 review，不需要 multi-angle 的複雜度。

5. **向後相容。** 現有的 `xreview review --files ... --context "..."` 完全不變。新功能是增量的。

---

## 待決問題

1. **Context-file 格式：** JSON vs YAML vs Markdown？JSON 在 Go 裡最容易 parse；YAML 更可讀；Markdown 可能對 Claude Code 最自然。建議：支援 JSON 和 YAML，讓 Claude Code 自己選。

2. **Merge 去重演算法：** 如何偵測兩個來自不同角度的 findings 描述同一個問題？用 file:line 鄰近度？用描述的語義相似度？先簡單做（同檔案、重疊行範圍、>70% 描述重疊 via fuzzy match），根據實際使用迭代。

3. **Session-group 儲存：** flat file `.xreview/groups/<id>.json` 列出成員 session ID？還是在每個 session 裡嵌入 group 資訊？建議：flat file——更簡單，避免修改 session schema。

4. **Skill 厚度：** Skill 應該規定多少 review 方法論 vs 讓 Claude Code 自己判斷？太規範 → 僵化、無法適應。太開放 → 行為不一致。先規範（明確的決策樹），隨著學到 Claude Code 自主處理得好的部分再放寬。

5. **Context-file 大小：** Claude Code 為大型 codebase 準備完整 context-file 本身可能很耗資源（大量 grep/read 操作）。Skill 是否應該設定 context 深度指南（例如「追蹤 symbol 最多 2 hop」、「上限 50 個 cross-reference」）？可能需要——需要實驗找到正確的平衡。
