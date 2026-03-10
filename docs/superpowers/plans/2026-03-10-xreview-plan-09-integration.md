# Plan 9: Integration Tests with Mock Codex

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a mock codex binary and integration tests that exercise the full lifecycle without a real codex instance.

**Architecture:** Mock codex is a shell script that routes responses based on prompt content. Integration tests build xreview, prepend mock to PATH, and run full command sequences.

**Tech Stack:** Bash (mock), Go testing, `os/exec`

**Depends on:** Plans 1-8 (full CLI must be working)

---

## Chunk 1: Mock Codex + Integration Test Fixtures

### File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `test/mock-codex/codex` | Mock codex binary (bash script) |
| Create | `test/fixtures/codex-output/review-response.json` | Standard review response |
| Create | `test/fixtures/codex-output/verify-response.json` | Verify/resume response |
| Create | `test/fixtures/codex-output/approved-response.json` | Clean code response |
| Create | `test/integration/integration_test.go` | Integration tests |

---

### Task 9.1: Mock Codex Binary + Response Fixtures

**Files:**
- Create: `test/mock-codex/codex`
- Create: `test/fixtures/codex-output/review-response.json`
- Create: `test/fixtures/codex-output/verify-response.json`
- Create: `test/fixtures/codex-output/approved-response.json`

- [ ] **Step 1: Create review-response.json**

Create `test/fixtures/codex-output/review-response.json`:

```json
{
  "verdict": "REVISE",
  "summary": "Found 2 issues",
  "findings": [
    {
      "id": "F001",
      "severity": "high",
      "category": "security",
      "file": "src/auth.go",
      "line": 42,
      "description": "JWT token is not checked for expiration",
      "suggestion": "Add exp claim validation after jwt.Parse"
    },
    {
      "id": "F002",
      "severity": "medium",
      "category": "logic",
      "file": "src/middleware.go",
      "line": 15,
      "description": "Error returned without context wrapping",
      "suggestion": "Use fmt.Errorf to wrap the error"
    }
  ]
}
```

- [ ] **Step 2: Create verify-response.json**

Create `test/fixtures/codex-output/verify-response.json`:

```json
{
  "verdict": "REVISE",
  "summary": "1 issue fixed, 1 dismissed, 1 still open",
  "findings": [
    {
      "id": "F001",
      "severity": "high",
      "category": "security",
      "status": "fixed",
      "file": "src/auth.go",
      "line": 42,
      "description": "JWT token is not checked for expiration",
      "suggestion": "Add exp claim validation",
      "verification_note": "Fix confirmed. Expiration check added correctly."
    },
    {
      "id": "F002",
      "severity": "medium",
      "category": "logic",
      "status": "dismissed",
      "file": "src/middleware.go",
      "line": 15,
      "description": "Error returned without context wrapping",
      "suggestion": "Use fmt.Errorf to wrap the error",
      "verification_note": "Dismissal reasonable. Caller wraps the error."
    }
  ]
}
```

- [ ] **Step 3: Create approved-response.json**

Create `test/fixtures/codex-output/approved-response.json`:

```json
{
  "verdict": "APPROVED",
  "summary": "No issues found",
  "findings": []
}
```

- [ ] **Step 4: Create mock codex binary**

Create `test/mock-codex/codex`:

```bash
#!/bin/bash
# Mock codex binary for integration testing.
# Routes based on command and prompt content.

FIXTURES_DIR="$(cd "$(dirname "$0")/../fixtures/codex-output" && pwd)"

# Emit a fake session ID on stderr (simulating real codex behavior)
echo "Session: a1b2c3d4-e5f6-7890-abcd-ef1234567890" >&2

# Handle auth status check
if [[ "$1" == "auth" && "$2" == "status" ]]; then
    echo '{"status": "authenticated", "user": "test@example.com"}'
    exit 0
fi

# Handle exec command
if [[ "$1" == "exec" ]]; then
    # Get the last argument (prompt)
    prompt="${@: -1}"

    # Route: preflight responsiveness check
    if echo "$prompt" | grep -qi "respond with OK"; then
        echo "OK"
        exit 0
    fi

    # Route: resume/verify round
    if echo "$prompt" | grep -qi "follow-up review"; then
        cat "$FIXTURES_DIR/verify-response.json"
        exit 0
    fi

    # Route: approved (clean code)
    if echo "$prompt" | grep -qi "README"; then
        cat "$FIXTURES_DIR/approved-response.json"
        exit 0
    fi

    # Default: first round review response
    cat "$FIXTURES_DIR/review-response.json"
    exit 0
fi

echo "Unknown command: $@" >&2
exit 1
```

- [ ] **Step 5: Make mock executable and commit**

```bash
chmod +x test/mock-codex/codex
git add test/mock-codex/ test/fixtures/codex-output/review-response.json test/fixtures/codex-output/verify-response.json test/fixtures/codex-output/approved-response.json
git commit -m "test: add mock codex binary and response fixtures"
```

---

### Task 9.2: Integration Tests

**Files:**
- Create: `test/integration/integration_test.go`

- [ ] **Step 1: Write integration tests**

Create `test/integration/integration_test.go`:

```go
//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var xreviewBin string

func TestMain(m *testing.M) {
	// Build xreview binary
	tmpDir, _ := os.MkdirTemp("", "xreview-integration-*")
	xreviewBin = filepath.Join(tmpDir, "xreview")

	buildCmd := exec.Command("go", "build", "-o", xreviewBin, "../../cmd/xreview")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		panic("build failed: " + string(out))
	}

	// Prepend mock codex to PATH
	mockDir, _ := filepath.Abs("../mock-codex")
	os.Setenv("PATH", mockDir+":"+os.Getenv("PATH"))

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

func runXReview(t *testing.T, workdir string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(xreviewBin, append([]string{"--workdir", workdir}, args...)...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestVersion(t *testing.T) {
	stdout, _, _ := runXReview(t, t.TempDir(), "version")
	if !strings.Contains(stdout, `action="version"`) {
		t.Errorf("unexpected output: %s", stdout)
	}
	if !strings.Contains(stdout, `status="success"`) {
		t.Errorf("expected success status: %s", stdout)
	}
}

func TestPreflight(t *testing.T) {
	stdout, _, _ := runXReview(t, t.TempDir(), "preflight")
	if !strings.Contains(stdout, `action="preflight"`) {
		t.Errorf("unexpected output: %s", stdout)
	}
	if !strings.Contains(stdout, `name="codex_installed"`) {
		t.Errorf("missing codex_installed check: %s", stdout)
	}
}

func TestReview_NewSession(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "auth.go"), []byte("package main\nfunc auth() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "middleware.go"), []byte("package main\nfunc mid() {}\n"), 0644)

	stdout, _, err := runXReview(t, dir, "review",
		"--files", "src/auth.go,src/middleware.go",
		"--context", "test review")

	if err != nil {
		t.Fatalf("review failed: %v\nstdout: %s", err, stdout)
	}

	if !strings.Contains(stdout, `action="review"`) {
		t.Errorf("missing action: %s", stdout)
	}
	if !strings.Contains(stdout, `round="1"`) {
		t.Errorf("missing round: %s", stdout)
	}
	if !strings.Contains(stdout, `id="F001"`) {
		t.Errorf("missing F001: %s", stdout)
	}

	// Verify session directory was created
	sessionsDir := filepath.Join(dir, ".xreview", "sessions")
	entries, _ := os.ReadDir(sessionsDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 session, got %d", len(entries))
	}
}

func TestReview_FullLifecycle(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "auth.go"), []byte("package main\nfunc auth() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "middleware.go"), []byte("package main\nfunc mid() {}\n"), 0644)

	// Step 1: New review
	stdout, _, err := runXReview(t, dir, "review",
		"--files", "src/auth.go,src/middleware.go",
		"--context", "test")
	if err != nil {
		t.Fatalf("review: %v", err)
	}

	// Extract session ID from output
	sessionID := extractSessionID(t, stdout)

	// Step 2: Resume review
	stdout, _, err = runXReview(t, dir, "review",
		"--session", sessionID,
		"--message", "Fixed F001, dismissed F002")
	if err != nil {
		t.Fatalf("resume: %v\nstdout: %s", err, stdout)
	}
	if !strings.Contains(stdout, `round="2"`) {
		t.Errorf("expected round 2: %s", stdout)
	}

	// Step 3: Report
	stdout, _, err = runXReview(t, dir, "report", "--session", sessionID)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if !strings.Contains(stdout, `action="report"`) {
		t.Errorf("missing report action: %s", stdout)
	}

	// Verify report file exists
	reportPath := filepath.Join(dir, ".xreview", "sessions", sessionID, "report.md")
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Error("report.md not created")
	}

	// Step 4: Clean
	stdout, _, err = runXReview(t, dir, "clean", "--session", sessionID)
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if !strings.Contains(stdout, `action="clean"`) {
		t.Errorf("missing clean action: %s", stdout)
	}

	// Verify session directory deleted
	sessDir := filepath.Join(dir, ".xreview", "sessions", sessionID)
	if _, err := os.Stat(sessDir); !os.IsNotExist(err) {
		t.Error("session directory not deleted")
	}
}

func TestReview_InvalidFlags(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "files and git-uncommitted",
			args: []string{"review", "--files", "a.go", "--git-uncommitted"},
			want: "INVALID_FLAGS",
		},
		{
			name: "session with files",
			args: []string{"review", "--session", "xr-123", "--files", "a.go"},
			want: "INVALID_FLAGS",
		},
		{
			name: "message without session",
			args: []string{"review", "--message", "fixed stuff"},
			want: "INVALID_FLAGS",
		},
		{
			name: "no targets",
			args: []string{"review"},
			want: "INVALID_FLAGS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, _ := runXReview(t, dir, tt.args...)
			if !strings.Contains(stdout, tt.want) {
				t.Errorf("expected %s in output: %s", tt.want, stdout)
			}
		})
	}
}

func TestReview_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	stdout, _, _ := runXReview(t, dir, "review", "--files", "nonexistent.go")
	if !strings.Contains(stdout, "FILE_NOT_FOUND") {
		t.Errorf("expected FILE_NOT_FOUND: %s", stdout)
	}
}

func TestReview_SessionNotFound(t *testing.T) {
	dir := t.TempDir()
	stdout, _, _ := runXReview(t, dir, "review", "--session", "xr-99999999-ffffff", "--message", "test")
	if !strings.Contains(stdout, "SESSION_NOT_FOUND") {
		t.Errorf("expected SESSION_NOT_FOUND: %s", stdout)
	}
}

func TestClean_SessionNotFound(t *testing.T) {
	dir := t.TempDir()
	stdout, _, _ := runXReview(t, dir, "clean", "--session", "xr-99999999-ffffff")
	if !strings.Contains(stdout, "SESSION_NOT_FOUND") {
		t.Errorf("expected SESSION_NOT_FOUND: %s", stdout)
	}
}

func extractSessionID(t *testing.T, xml string) string {
	t.Helper()
	// Find session="xr-XXXXXXXX-XXXXXX"
	idx := strings.Index(xml, `session="xr-`)
	if idx < 0 {
		t.Fatalf("no session ID in output: %s", xml)
	}
	start := idx + len(`session="`)
	end := strings.Index(xml[start:], `"`)
	if end < 0 {
		t.Fatalf("malformed session attribute: %s", xml[start:])
	}
	return xml[start : start+end]
}
```

- [ ] **Step 2: Run integration tests**

```bash
cd /home/davidleitw/xreview && go test ./test/integration/ -tags=integration -v -timeout 60s
```

Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add test/integration/
git commit -m "test: add integration tests with mock codex for full lifecycle"
```
