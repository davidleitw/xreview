# xreview Brainstorm Decisions

**Date:** 2026-03-10
**Status:** Confirmed

以下為 brainstorming session 中確認的設計決策，已反映至主設計文件。

---

## Decision 1: Reviewer Backend — Codex 優先，Interface 保留

- Day 1 只支援 Codex CLI 作為 review backend
- 保留 `Reviewer` interface（`SingleReviewer` / 未來 `MultiReviewer`）
- CLI 介面不因 backend 切換而改變

## Decision 2: --context 結構化內容，不用臨時檔案

- `--context` 維持單一字串 flag，不新增 `--context-file`
- Skill 引導 Claude Code 組出結構化格式：
  - 【變更類型】feature / refactor / bugfix
  - 【描述】做了什麼
  - 【預期行為】應達成的效果（refactor 則寫「行為應與修改前一致」）
- xreview 不 parse 這些標籤，直接整段塞進 codex prompt
- 避免臨時 JSON 檔案殘留問題

## Decision 3: Review 全面性由 xreview prompt 保底

- review 的全面性（安全性、可讀性、維護性、擴充性）由 xreview 的 prompt template 保證
- 不提供 `focus_areas` / `skip_areas` 讓呼叫方選擇性跳過
- codex 的建議應以 scope 內可落地的改善為主，不建議大規模重寫

## Decision 4: 三方共識交互模式

- **三方**：Codex（審查者）、Claude Code（執行者）、使用者（決策者）
- 對每個 finding，Claude Code 用 `AskUserQuestion` 逐條詢問使用者：
  - 說明 finding 內容（白話）
  - 給出 Claude Code 的建議和理由
  - 提供選項，**一定包含「不修」**：
    - (a) 依建議修復
    - (b) 用其他方式修（使用者說明）
    - (c) 不修（使用者說明理由，傳給 codex 評估）
- 修復後呼叫 `xreview review --session <id> --message "..."`
- 如果 codex 仍有異議，回到逐條確認
- 重複直到三方共識或達 5 輪上限

## Decision 5: --message 保持自然語言

- `--message` 維持單一字串，不結構化
- codex 有 `--resume` 記憶，讀得懂自然語言
- 未來有需要再演進

## Decision 6: --files 支援檔案與目錄

- `--files` 同時接受檔案路徑和目錄路徑
- 目錄自動遞迴展開，尊重 `ignore_patterns`
- 不另外加 `--file` / `--folder` flag

## Decision 7: 簡化 Session 儲存

- 移除 `rounds/` 目錄（不需要每輪快照）
- 移除獨立的 `findings.json`
- 每個 session 只有一份 `session.json`，每輪直接更新
- codex 的 `--resume` 已保留完整記憶，不需 xreview 側重播歷史

## Decision 8: Schema 內嵌 Binary

- JSON schema 不存在 session 目錄
- 用 `//go:embed` 嵌入 Go binary
- 執行時寫到 `os.TempDir()`，codex 讀取後刪除
- `.xreview/` 結構極簡：只有 `sessions/<id>/session.json`
