# Plan 1: Project Scaffold + Core Types

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bootstrap the Go project with module, core type definitions, config loading, and build tooling.

**Architecture:** Standard Go project layout with `cmd/xreview/` entry point and `internal/` packages. Cobra for CLI framework. Minimal external dependencies (only cobra).

**Tech Stack:** Go 1.22+, github.com/spf13/cobra

---

## Chunk 1: Go Module + Core Types + Config + Makefile

### File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `go.mod` | Module definition |
| Create | `cmd/xreview/main.go` | Entry point (minimal, just root command) |
| Create | `internal/session/types.go` | All shared structs: Session, Finding, Round, FindingSummary, Config |
| Create | `internal/session/types_test.go` | JSON marshal/unmarshal round-trip tests |
| Create | `internal/config/config.go` | Load `.xreview/config.json` with defaults |
| Create | `internal/config/config_test.go` | Config loading tests |
| Create | `internal/version/version.go` | Version constants + build-time injection |
| Create | `Makefile` | build, test, lint, install targets |

---

### Task 1.1: Initialize Go Module

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Create go.mod**

```bash
cd /home/davidleitw/xreview && go mod init github.com/davidleitw/xreview
```

- [ ] **Step 2: Add cobra dependency**

```bash
cd /home/davidleitw/xreview && go get github.com/spf13/cobra@latest
```

- [ ] **Step 3: Verify go.mod exists**

```bash
cat go.mod
```

Expected: module path `github.com/davidleitw/xreview`, require `github.com/spf13/cobra`

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: init go module with cobra dependency"
```

---

### Task 1.2: Core Type Definitions

**Files:**
- Create: `internal/session/types.go`
- Test: `internal/session/types_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/session/types_test.go`:

```go
package session

import (
	"encoding/json"
	"testing"
)

func TestSessionJSON_RoundTrip(t *testing.T) {
	s := Session{
		SessionID:      "xr-20260310-a1b2c3",
		XReviewVersion: "0.1.0",
		CreatedAt:      "2026-03-10T14:30:00Z",
		UpdatedAt:      "2026-03-10T14:45:00Z",
		Status:         StatusInReview,
		CurrentRound:   2,
		CodexSessionID: "cs-xxxxxx",
		CodexModel:     "gpt-5.4",
		Context:        "Implemented JWT authentication",
		Targets:        []string{"src/auth.go", "src/middleware.go"},
		TargetMode:     "files",
		Config:         SessionConfig{Timeout: 180},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Session
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.SessionID != s.SessionID {
		t.Errorf("session_id: got %q, want %q", got.SessionID, s.SessionID)
	}
	if got.Status != StatusInReview {
		t.Errorf("status: got %q, want %q", got.Status, StatusInReview)
	}
	if got.CurrentRound != 2 {
		t.Errorf("current_round: got %d, want 2", got.CurrentRound)
	}
	if len(got.Targets) != 2 {
		t.Errorf("targets len: got %d, want 2", len(got.Targets))
	}
}

func TestFindingJSON_RoundTrip(t *testing.T) {
	f := Finding{
		ID:               "F001",
		Severity:         "high",
		Category:         "security",
		Status:           FindingOpen,
		File:             "src/auth.go",
		Line:             42,
		Description:      "JWT token is not checked for expiration.",
		Suggestion:       "Add exp claim validation.",
		CodeSnippet:      "token, err := jwt.Parse(rawToken, keyFunc)",
		FirstSeenRound:   1,
		LastUpdatedRound: 2,
		History: []FindingHistoryEntry{
			{Round: 1, Status: FindingOpen, Note: "initial finding"},
			{Round: 2, Status: FindingFixed, Note: "expiration check added"},
		},
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Finding
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != "F001" {
		t.Errorf("id: got %q, want %q", got.ID, "F001")
	}
	if got.Status != FindingOpen {
		t.Errorf("status: got %q, want %q", got.Status, FindingOpen)
	}
	if len(got.History) != 2 {
		t.Errorf("history len: got %d, want 2", len(got.History))
	}
}

func TestRoundJSON_RoundTrip(t *testing.T) {
	r := Round{
		RoundNum:       1,
		Timestamp:      "2026-03-10T14:30:05Z",
		Action:         "review",
		CodexSessionID: "cs-xxxxxx",
		CodexResumed:   false,
		FullRescan:     false,
		UserMessage:    "",
		Targets:        []string{"src/auth.go"},
		FindingsBefore: FindingSummary{Total: 0},
		FindingsAfter:  FindingSummary{Total: 3, Open: 3},
		RawStdoutPath:  "raw/round-001-codex-stdout.txt",
		RawStderrPath:  "raw/round-001-codex-stderr.txt",
		DurationMs:     8500,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Round
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.RoundNum != 1 {
		t.Errorf("round: got %d, want 1", got.RoundNum)
	}
	if got.DurationMs != 8500 {
		t.Errorf("duration_ms: got %d, want 8500", got.DurationMs)
	}
}

func TestFindingsFileJSON_RoundTrip(t *testing.T) {
	ff := FindingsFile{
		LastUpdatedRound: 2,
		Findings: []Finding{
			{
				ID:       "F001",
				Severity: "high",
				Status:   FindingFixed,
			},
		},
		Summary: FindingSummary{Total: 1, Fixed: 1},
	}

	data, err := json.Marshal(ff)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got FindingsFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.LastUpdatedRound != 2 {
		t.Errorf("last_updated_round: got %d, want 2", got.LastUpdatedRound)
	}
	if len(got.Findings) != 1 {
		t.Errorf("findings len: got %d, want 1", len(got.Findings))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v -run TestSession`
Expected: FAIL — types not defined

- [ ] **Step 3: Write types.go**

Create `internal/session/types.go`:

```go
package session

// Session status constants.
const (
	StatusInitialized = "initialized"
	StatusInReview    = "in_review"
	StatusVerifying   = "verifying"
	StatusCompleted   = "completed"
)

// Finding status constants.
const (
	FindingOpen      = "open"
	FindingFixed     = "fixed"
	FindingDismissed = "dismissed"
	FindingReopened  = "reopened"
)

// Session represents the core session state stored in session.json.
type Session struct {
	SessionID      string        `json:"session_id"`
	XReviewVersion string        `json:"xreview_version"`
	CreatedAt      string        `json:"created_at"`
	UpdatedAt      string        `json:"updated_at"`
	Status         string        `json:"status"`
	CurrentRound   int           `json:"current_round"`
	CodexSessionID string        `json:"codex_session_id"`
	CodexModel     string        `json:"codex_model"`
	Context        string        `json:"context"`
	Targets        []string      `json:"targets"`
	TargetMode     string        `json:"target_mode"`
	Config         SessionConfig `json:"config"`
}

// SessionConfig holds runtime config for a session.
type SessionConfig struct {
	Timeout int `json:"timeout"`
}

// Finding represents a single review finding.
type Finding struct {
	ID               string                `json:"id"`
	Severity         string                `json:"severity"`
	Category         string                `json:"category"`
	Status           string                `json:"status"`
	File             string                `json:"file"`
	Line             int                   `json:"line"`
	Description      string                `json:"description"`
	Suggestion       string                `json:"suggestion"`
	CodeSnippet      string                `json:"code_snippet"`
	FirstSeenRound   int                   `json:"first_seen_round"`
	LastUpdatedRound int                   `json:"last_updated_round"`
	History          []FindingHistoryEntry `json:"history"`
	VerificationNote string                `json:"verification_note,omitempty"`
	Comparison       string                `json:"comparison,omitempty"`
}

// FindingHistoryEntry tracks a finding's status change in a round.
type FindingHistoryEntry struct {
	Round  int    `json:"round"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

// FindingsFile represents the findings.json file.
type FindingsFile struct {
	LastUpdatedRound int            `json:"last_updated_round"`
	Findings         []Finding      `json:"findings"`
	Summary          FindingSummary `json:"summary"`
}

// FindingSummary holds aggregated finding counts.
type FindingSummary struct {
	Total     int `json:"total"`
	Open      int `json:"open"`
	Fixed     int `json:"fixed"`
	Dismissed int `json:"dismissed"`
}

// Round represents a single review round stored in round-NNN.json.
type Round struct {
	RoundNum       int            `json:"round"`
	Timestamp      string         `json:"timestamp"`
	Action         string         `json:"action"`
	CodexSessionID string         `json:"codex_session_id"`
	CodexResumed   bool           `json:"codex_resumed"`
	FullRescan     bool           `json:"full_rescan"`
	UserMessage    string         `json:"user_message"`
	Targets        []string       `json:"targets_snapshot"`
	FindingsBefore FindingSummary `json:"findings_before"`
	FindingsAfter  FindingSummary `json:"findings_after"`
	RawStdoutPath  string         `json:"raw_stdout_path"`
	RawStderrPath  string         `json:"raw_stderr_path"`
	DurationMs     int64          `json:"duration_ms"`
	ResumeFallback string         `json:"resume_fallback_reason,omitempty"`
}

// CodexResponse represents the JSON output from codex (parsed by parser package).
type CodexResponse struct {
	Verdict  string         `json:"verdict"`
	Summary  string         `json:"summary"`
	Findings []CodexFinding `json:"findings"`
}

// CodexFinding represents a single finding from codex output.
type CodexFinding struct {
	ID               string `json:"id,omitempty"`
	Severity         string `json:"severity"`
	Category         string `json:"category,omitempty"`
	File             string `json:"file,omitempty"`
	Line             int    `json:"line,omitempty"`
	Description      string `json:"description"`
	Suggestion       string `json:"suggestion"`
	CodeSnippet      string `json:"code_snippet,omitempty"`
	Status           string `json:"status,omitempty"`
	VerificationNote string `json:"verification_note,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v`
Expected: All 4 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/session/types.go internal/session/types_test.go
git commit -m "feat: add core type definitions for session, finding, round"
```

---

### Task 1.3: Config Package

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Default(t *testing.T) {
	// No config file — should return defaults
	cfg, err := Load("/nonexistent/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CodexModel != DefaultCodexModel {
		t.Errorf("codex_model: got %q, want %q", cfg.CodexModel, DefaultCodexModel)
	}
	if cfg.DefaultTimeout != DefaultTimeout {
		t.Errorf("default_timeout: got %d, want %d", cfg.DefaultTimeout, DefaultTimeout)
	}
}

func TestLoadConfig_FromFile(t *testing.T) {
	dir := t.TempDir()
	xreviewDir := filepath.Join(dir, ".xreview")
	os.MkdirAll(xreviewDir, 0755)

	data := []byte(`{
		"codex_model": "gpt-5.4",
		"default_timeout": 300,
		"ignore_patterns": ["**/*_test.go"]
	}`)
	os.WriteFile(filepath.Join(xreviewDir, "config.json"), data, 0644)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CodexModel != "gpt-5.4" {
		t.Errorf("codex_model: got %q, want %q", cfg.CodexModel, "gpt-5.4")
	}
	if cfg.DefaultTimeout != 300 {
		t.Errorf("default_timeout: got %d, want 300", cfg.DefaultTimeout)
	}
	if len(cfg.IgnorePatterns) != 1 || cfg.IgnorePatterns[0] != "**/*_test.go" {
		t.Errorf("ignore_patterns: got %v", cfg.IgnorePatterns)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	xreviewDir := filepath.Join(dir, ".xreview")
	os.MkdirAll(xreviewDir, 0755)
	os.WriteFile(filepath.Join(xreviewDir, "config.json"), []byte("{invalid"), 0644)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/davidleitw/xreview && go test ./internal/config/ -v`
Expected: FAIL — package not defined

- [ ] **Step 3: Write config.go**

Create `internal/config/config.go`:

```go
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	DefaultCodexModel = "o3"
	DefaultTimeout    = 180
)

// Config represents .xreview/config.json.
type Config struct {
	CodexModel     string   `json:"codex_model"`
	DefaultTimeout int      `json:"default_timeout"`
	DefaultContext string   `json:"default_context"`
	IgnorePatterns []string `json:"ignore_patterns"`
}

// Load reads .xreview/config.json from workdir. Returns defaults if file does not exist.
func Load(workdir string) (*Config, error) {
	cfg := &Config{
		CodexModel:     DefaultCodexModel,
		DefaultTimeout: DefaultTimeout,
	}

	path := filepath.Join(workdir, ".xreview", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Apply defaults for zero values
	if cfg.CodexModel == "" {
		cfg.CodexModel = DefaultCodexModel
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = DefaultTimeout
	}

	return cfg, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/config/ -v`
Expected: All 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config package with .xreview/config.json loading"
```

---

### Task 1.4: Version Package

**Files:**
- Create: `internal/version/version.go`

- [ ] **Step 1: Write version.go**

Create `internal/version/version.go`:

```go
package version

// Set via ldflags at build time:
//   go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=0.1.0"
var (
	Version = "dev"
)
```

- [ ] **Step 2: Commit**

```bash
git add internal/version/version.go
git commit -m "feat: add version package with build-time injection"
```

---

### Task 1.5: Minimal main.go + Makefile

**Files:**
- Create: `cmd/xreview/main.go`
- Create: `Makefile`

- [ ] **Step 1: Write main.go**

Create `cmd/xreview/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagWorkdir string
	flagVerbose bool
	flagJSON    bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "xreview",
		Short: "Agent-Native Code Review Engine",
		Long:  "xreview orchestrates code review between Claude Code and Codex.",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&flagWorkdir, "workdir", "", "Override working directory (default: current directory)")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Print debug information to stderr")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output raw JSON instead of XML")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Write Makefile**

Create `Makefile`:

```makefile
VERSION ?= dev

.PHONY: build test test-integration test-e2e lint install clean

build:
	go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o bin/xreview ./cmd/xreview

test:
	go test ./internal/... ./cmd/...

test-integration:
	go test ./test/... -tags=integration

test-e2e:
	XREVIEW_E2E=1 go test ./test/e2e/... -tags=e2e -timeout 300s

lint:
	golangci-lint run ./...

install:
	go install -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" ./cmd/xreview

clean:
	rm -rf bin/
```

- [ ] **Step 3: Verify build**

```bash
cd /home/davidleitw/xreview && make build && ./bin/xreview --help
```

Expected: Help text with "Agent-Native Code Review Engine"

- [ ] **Step 4: Commit**

```bash
git add cmd/xreview/main.go Makefile
git commit -m "feat: add minimal CLI entry point and Makefile"
```
