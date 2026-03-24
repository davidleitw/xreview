# Timeout 預設值調整與 Duration 格式支援

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 將 `--timeout` 預設從 180s 改為 600s (10m)，並支援 duration 字串格式（`5m`、`10m30s`），同時改善 error message 讓使用者清楚知道預設值和如何調整。

**Architecture:** 新增 `parseDuration` helper 處理兩種格式（純數字當秒、Go duration 字串）。常數與 flag 預設值統一改為 600。Error message 加上預設值資訊。

**Tech Stack:** Go, cobra flags, `time.ParseDuration`

---

## 改動範圍

| 檔案 | 動作 | 說明 |
|------|------|------|
| `internal/config/config.go:11` | Modify | `DefaultTimeout` 180 → 600 |
| `cmd/xreview/cmd_review.go` | Modify | flag 型別 `int` → `string`，預設 `"10m"`，加 `parseDuration`，加 `strconv`/`time` import |
| `internal/codex/runner.go:116` | Modify | 改善 timeout error message |
| `internal/formatter/formatter_test.go:11,16` | Modify | 更新 error message 斷言 |
| `internal/reviewer/single_test.go` | Modify | 9 處 `DefaultTimeout: 180` → `600` |
| `internal/session/manager_test.go:13` | Modify | `DefaultTimeout: 180` → `600` |

不需要改的檔案：
- `cmd/xreview/cmd_review_test.go` — 現有 tests 不涉及 timeout，不受影響
- `internal/codex/runner_test.go` — `TestExecRequest_TimeoutField` 直接用 `time.Duration`，不受影響

---

### Task 1: 修改 DefaultTimeout 常數

**Files:**
- Modify: `internal/config/config.go:11`

- [ ] **Step 1: 改 DefaultTimeout**

```go
DefaultTimeout = 600
```

- [ ] **Step 2: 跑現有 config tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/config/ -v`
Expected: PASS

---

### Task 2: `--timeout` flag 支援 duration 格式

**Files:**
- Modify: `cmd/xreview/cmd_review.go`

需要加入的 import：`"strconv"` 和 `"time"`（`"fmt"` 和 `"strings"` 已存在）

- [ ] **Step 1: 改 flag 型別、加 import、加 parseDuration**

1. 在 import block 加入 `"strconv"` 和 `"time"`
2. 將 var block 中 `timeout int` 改為 `timeout string`
3. 在檔案底部（`classifyReviewError` 前）加入：

```go
// parseDuration parses a timeout string. Accepts Go duration format ("5m", "10m30s")
// or plain integer (treated as seconds for backward compatibility).
func parseDuration(s string) (int, error) {
	if secs, err := strconv.Atoi(s); err == nil {
		if secs <= 0 {
			return 0, fmt.Errorf("timeout must be positive, got %d", secs)
		}
		return secs, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q: use seconds (e.g. 300) or duration (e.g. 5m, 10m30s)", s)
	}
	secs := int(d.Seconds())
	if secs <= 0 {
		return 0, fmt.Errorf("timeout must be positive, got %s", s)
	}
	return secs, nil
}
```

- [ ] **Step 2: 在 RunE 開頭解析 timeout**

在 `RunE` 函式中，`cfg, err := config.Load(flagWorkdir)` 之前加入：

```go
timeoutSecs, err := parseDuration(timeout)
if err != nil {
	return printErr("review", formatter.ErrInvalidFlags, err)
}
```

然後把 RunE 中所有 `Timeout: timeout` 改為 `Timeout: timeoutSecs`（共 2 處：line 86 和 line 122）。

- [ ] **Step 3: 更新 flag 定義**

將 line 142：
```go
cmd.Flags().IntVar(&timeout, "timeout", 180, "Timeout in seconds for codex response")
```
改為：
```go
cmd.Flags().StringVar(&timeout, "timeout", "10m", "Timeout for codex response (e.g. 5m, 10m30s, 300)")
```

- [ ] **Step 4: 確認編譯通過**

Run: `cd /home/davidleitw/xreview && go build ./cmd/xreview/ && echo "build ok"`
Expected: build ok

---

### Task 3: 改善 timeout error message

**Files:**
- Modify: `internal/codex/runner.go:116`

- [ ] **Step 1: 更新 error message**

將 line 116 的 error message 改為：

```go
return result, fmt.Errorf(
	"codex did not respond within %s (default: 10m). "+
		"Try --timeout 15m or --timeout 20m for large reviews, "+
		"or reduce the number of files",
	req.Timeout,
)
```

- [ ] **Step 2: 跑 codex runner tests**

Run: `cd /home/davidleitw/xreview && go test ./internal/codex/ -v`
Expected: PASS（`runner_test.go` 不測 error message 內容，不受影響）

---

### Task 4: 更新所有 test 中的 DefaultTimeout

**Files:**
- Modify: `internal/reviewer/single_test.go` — 9 處 `DefaultTimeout: 180` → `600`（lines: 153, 199, 222, 277, 340, 356, 576, 690, 744）
- Modify: `internal/session/manager_test.go:13` — `DefaultTimeout: 180` → `600`
- Modify: `internal/formatter/formatter_test.go:11,16` — 更新 error message 字串

- [ ] **Step 1: 批量替換 single_test.go**

所有 `DefaultTimeout: 180` → `DefaultTimeout: 600`（9 處）

- [ ] **Step 2: 替換 manager_test.go**

`DefaultTimeout: 180` → `DefaultTimeout: 600`（1 處）

- [ ] **Step 3: 更新 formatter_test.go**

Line 11 改為：
```go
result := FormatError("review", ErrCodexTimeout, "codex did not respond within 10m0s (default: 10m)")
```

Line 16 改為：
```go
assertContains(t, result, "codex did not respond within 10m0s (default: 10m)")
```

- [ ] **Step 4: 跑全部 tests**

Run: `cd /home/davidleitw/xreview && go test ./... -count=1`
Expected: ALL PASS

---

### Task 5: Commit

- [ ] **Step 1: Commit**

```bash
git add internal/config/config.go cmd/xreview/cmd_review.go internal/codex/runner.go \
  internal/reviewer/single_test.go internal/session/manager_test.go \
  internal/formatter/formatter_test.go
git commit -m "feat: change default timeout to 10m, support duration format (5m, 10m30s)"
```
