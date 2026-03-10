# xreview Project Scaffold Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bootstrap the xreview Go project from zero to a compilable binary where `xreview --help` shows all 6 subcommands, plus the Claude Code skill files.

**Architecture:** Go CLI with cobra, internal packages for each concern (collector, prompt, parser, formatter, session, codex, reviewer, schema, config, version). Leaf packages first, then interface packages, then CLI layer on top. Claude Code skill at `.claude/skills/xreview/`.

**Tech Stack:** Go 1.22+, github.com/spf13/cobra, Go stdlib (encoding/json, encoding/xml, os/exec, text/template)

**Source of truth:** `docs/specs/2026-03-10-xreview-design.md`, `docs/specs/2026-03-10-brainstorm-decisions.md`

---

### Task 1: Initialize Go Module + Support Files

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `Makefile`

**Step 1: Initialize Go module**

Run: `go mod init github.com/davidleitw/xreview`
Expected: `go.mod` created

**Step 2: Add cobra dependency**

Run: `go get github.com/spf13/cobra@latest`
Expected: `go.mod` has cobra require, `go.sum` created

**Step 3: Create .gitignore**

```
# Build output
bin/

# xreview session data
.xreview/

# Test binaries
*.test

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
```

**Step 4: Create Makefile**

```makefile
VERSION ?= dev

.PHONY: build test lint install clean

build:
	go build -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" -o bin/xreview ./cmd/xreview

test:
	go test ./internal/... ./cmd/...

lint:
	golangci-lint run ./...

install:
	go install -ldflags "-X github.com/davidleitw/xreview/internal/version.Version=$(VERSION)" ./cmd/xreview

clean:
	rm -rf bin/
```

**Step 5: Commit**

```bash
git add go.mod go.sum .gitignore Makefile
git commit -m "feat: initialize Go module with cobra dependency"
```

---

### Task 2: Leaf Packages (no internal dependencies)

**Files:**
- Create: `internal/version/version.go`
- Create: `internal/config/config.go`
- Create: `internal/session/types.go`

**Step 1: Create internal/version/version.go**

```go
package version

// Version is set at build time via -ldflags.
var Version = "dev"
```

**Step 2: Create internal/config/config.go**

```go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DefaultCodexModel  = "o3"
	DefaultTimeout     = 180
	ConfigFileName     = "config.json"
	SessionsDirName    = "sessions"
	XReviewDirName     = ".xreview"
)

// Config holds project-level xreview configuration from .xreview/config.json.
type Config struct {
	CodexModel     string   `json:"codex_model"`
	DefaultTimeout int      `json:"default_timeout"`
	DefaultContext string   `json:"default_context"`
	IgnorePatterns []string `json:"ignore_patterns"`
}

// Load reads .xreview/config.json from the given workdir.
// Returns default config if the file does not exist.
func Load(workdir string) (*Config, error) {
	cfg := &Config{
		CodexModel:     DefaultCodexModel,
		DefaultTimeout: DefaultTimeout,
		IgnorePatterns: []string{
			"**/*_test.go",
			"**/vendor/**",
			"**/*.generated.go",
		},
	}

	path := filepath.Join(workdir, XReviewDirName, ConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.CodexModel == "" {
		cfg.CodexModel = DefaultCodexModel
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = DefaultTimeout
	}

	return cfg, nil
}

// SessionsDir returns the path to .xreview/sessions/ under the given workdir.
func SessionsDir(workdir string) string {
	return filepath.Join(workdir, XReviewDirName, SessionsDirName)
}
```

**Step 3: Create internal/session/types.go**

```go
package session

import "time"

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

// Session represents the complete state of a review session.
// Stored as a single session.json file, updated in-place each round.
type Session struct {
	SessionID      string    `json:"session_id"`
	XReviewVersion string    `json:"xreview_version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Status         string    `json:"status"`
	Round          int       `json:"round"`
	CodexSessionID string    `json:"codex_session_id,omitempty"`
	CodexModel     string    `json:"codex_model"`
	Context        string    `json:"context"`
	Targets        []string  `json:"targets"`
	TargetMode     string    `json:"target_mode"`
	Findings       []Finding `json:"findings"`
}

// Finding represents a single review finding.
type Finding struct {
	ID               string `json:"id"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Status           string `json:"status"`
	File             string `json:"file"`
	Line             int    `json:"line"`
	Description      string `json:"description"`
	Suggestion       string `json:"suggestion"`
	CodeSnippet      string `json:"code_snippet,omitempty"`
	VerificationNote string `json:"verification_note,omitempty"`
}

// FindingSummary holds aggregated counts of finding statuses.
type FindingSummary struct {
	Total     int `json:"total"`
	Open      int `json:"open"`
	Fixed     int `json:"fixed"`
	Dismissed int `json:"dismissed"`
}

// Summarize computes a FindingSummary from the current findings.
func (s *Session) Summarize() FindingSummary {
	sum := FindingSummary{Total: len(s.Findings)}
	for _, f := range s.Findings {
		switch f.Status {
		case FindingOpen, FindingReopened:
			sum.Open++
		case FindingFixed:
			sum.Fixed++
		case FindingDismissed:
			sum.Dismissed++
		}
	}
	return sum
}

// CodexResponse represents the structured JSON output from codex.
type CodexResponse struct {
	Verdict  string         `json:"verdict"`
	Summary  string         `json:"summary"`
	Findings []CodexFinding `json:"findings"`
}

// CodexFinding is a single finding as returned by codex JSON output.
type CodexFinding struct {
	ID               string `json:"id"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	File             string `json:"file"`
	Line             int    `json:"line"`
	Description      string `json:"description"`
	Suggestion       string `json:"suggestion"`
	CodeSnippet      string `json:"code_snippet,omitempty"`
	Status           string `json:"status,omitempty"`
	VerificationNote string `json:"verification_note,omitempty"`
}
```

**Step 4: Verify leaf packages compile**

Run: `go build ./internal/version/... ./internal/config/... ./internal/session/...`
Expected: no errors

**Step 5: Commit**

```bash
git add internal/version/ internal/config/ internal/session/
git commit -m "feat: add leaf packages — version, config, session types"
```

---

### Task 3: Schema Package (embedded JSON schema)

**Files:**
- Create: `internal/schema/review.json`
- Create: `internal/schema/schema.go`

**Step 1: Create internal/schema/review.json**

```json
{
  "type": "object",
  "properties": {
    "verdict": {
      "type": "string",
      "enum": ["APPROVED", "REVISE"],
      "description": "Overall review decision"
    },
    "summary": {
      "type": "string",
      "description": "Brief summary of review findings"
    },
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": { "type": "string" },
          "severity": { "type": "string", "enum": ["high", "medium", "low"] },
          "category": { "type": "string", "enum": ["security", "logic", "performance", "error-handling"] },
          "file": { "type": "string" },
          "line": { "type": "integer" },
          "description": { "type": "string" },
          "suggestion": { "type": "string" },
          "code_snippet": { "type": "string" },
          "status": { "type": "string", "enum": ["open", "fixed", "dismissed", "reopened"] },
          "verification_note": { "type": "string" }
        },
        "required": ["id", "severity", "description", "suggestion"],
        "additionalProperties": false
      }
    }
  },
  "required": ["verdict", "summary", "findings"],
  "additionalProperties": false
}
```

**Step 2: Create internal/schema/schema.go**

```go
package schema

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed review.json
var ReviewSchemaBytes []byte

// WriteTempSchema writes the embedded JSON schema to a temp file.
// Returns the file path and a cleanup function. Caller must call cleanup after use.
func WriteTempSchema() (path string, cleanup func(), err error) {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "xreview-schema-*.json")

	f, err := os.CreateTemp(tmpDir, "xreview-schema-*.json")
	if err != nil {
		return "", nil, fmt.Errorf("create temp schema file: %w", err)
	}

	if _, err := f.Write(ReviewSchemaBytes); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("write temp schema file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, fmt.Errorf("close temp schema file: %w", err)
	}

	_ = tmpFile // suppress unused warning from the initial assignment
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}
```

**Step 3: Verify compile**

Run: `go build ./internal/schema/...`
Expected: no errors

**Step 4: Commit**

```bash
git add internal/schema/
git commit -m "feat: add schema package with embedded codex output JSON schema"
```

---

### Task 4: Interface Packages (collector, prompt, parser, formatter)

**Files:**
- Create: `internal/collector/collector.go`
- Create: `internal/collector/git.go`
- Create: `internal/prompt/templates.go`
- Create: `internal/prompt/builder.go`
- Create: `internal/parser/extract.go`
- Create: `internal/parser/parser.go`
- Create: `internal/formatter/error.go`
- Create: `internal/formatter/xml.go`

**Step 1: Create internal/collector/collector.go**

```go
package collector

import (
	"context"
	"fmt"

	"github.com/davidleitw/xreview/internal/config"
)

// FileContent holds a file's path and its content with line numbers.
type FileContent struct {
	Path    string
	Content string
	Lines   int
}

// Collector reads file contents for review.
type Collector interface {
	// Collect reads the content of the given targets.
	// mode is "files" or "git-uncommitted".
	// Directories in targets are expanded recursively, respecting ignore patterns.
	Collect(ctx context.Context, targets []string, mode string) ([]FileContent, error)
}

type collector struct {
	cfg *config.Config
}

// NewCollector creates a Collector that respects the given config's ignore patterns.
func NewCollector(cfg *config.Config) Collector {
	return &collector{cfg: cfg}
}

func (c *collector) Collect(ctx context.Context, targets []string, mode string) ([]FileContent, error) {
	// TODO: implement — expand directories, read files, add line numbers
	return nil, fmt.Errorf("collector: not implemented")
}
```

**Step 2: Create internal/collector/git.go**

```go
package collector

import "fmt"

// GitUncommittedFiles returns the paths of all uncommitted files (staged + unstaged)
// in the git repository at workdir.
func GitUncommittedFiles(workdir string) ([]string, error) {
	// TODO: implement — run git diff --name-only + git diff --cached --name-only
	return nil, fmt.Errorf("git uncommitted: not implemented")
}
```

**Step 3: Create internal/prompt/templates.go**

```go
package prompt

// FirstRoundTemplate is the prompt template for the initial review round.
const FirstRoundTemplate = `<CRITICAL_RULES>
1. PERFORM STATIC ANALYSIS ONLY. Do NOT execute or run the code.
2. Only report issues you can directly observe in the provided code.
   Do NOT speculate about issues in code you cannot see.
3. Every finding MUST reference a specific file and line number.
4. Focus on real bugs and security issues. Do NOT report trivial style preferences.
5. If you find no issues, set verdict to APPROVED with an empty findings array.
6. You are encouraged to read additional files in the repository if needed
   to understand the full context of the code being reviewed.
7. Review comprehensively: security, correctness, readability, maintainability,
   and extensibility. Do NOT limit your review to a single aspect.
8. Suggestions MUST be scoped and actionable within the current change.
   Do NOT suggest large-scale rewrites or architectural overhauls.
   Focus on improvements that can be applied to the code being reviewed.
</CRITICAL_RULES>

You are a senior code reviewer. Analyze the following code changes for bugs,
security vulnerabilities, logic errors, and significant quality issues.

Context from the developer: {{.Context}}

===== FILES CHANGED =====

{{.FileList}}

===== DIFF =====

{{.Diff}}

===== END =====`

// ResumeTemplate is the prompt template for follow-up review rounds.
const ResumeTemplate = `This is a follow-up review. You previously reviewed these files and
identified the findings listed below. The developer has made changes
and provided the following update:

Developer message: "{{.Message}}"

===== PREVIOUS FINDINGS =====

{{.PreviousFindings}}

===== UPDATED FILES =====

{{.UpdatedFiles}}

===== END OF FILES =====

For each previous finding, determine:
1. If claimed fixed: verify the fix is actually correct and complete.
2. If claimed false positive: evaluate whether the dismissal is reasonable.
3. If no update: re-evaluate against the current code.

Also check: did any of the changes introduce NEW issues?

New findings (not in the previous list) should have status "open" and a new unique "id".`
```

**Step 4: Create internal/prompt/builder.go**

```go
package prompt

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/davidleitw/xreview/internal/session"
)

// FirstRoundInput holds the data for building a first-round prompt.
type FirstRoundInput struct {
	Context  string
	FileList string
	Diff     string
}

// ResumeInput holds the data for building a resume-round prompt.
type ResumeInput struct {
	Message          string
	PreviousFindings string
	UpdatedFiles     string
}

// Builder assembles prompts for codex.
type Builder interface {
	BuildFirstRound(input FirstRoundInput) (string, error)
	BuildResume(input ResumeInput) (string, error)
	// FormatFindingsForPrompt formats findings for inclusion in a resume prompt.
	FormatFindingsForPrompt(findings []session.Finding) string
}

type builder struct {
	firstRound *template.Template
	resume     *template.Template
}

// NewBuilder creates a Builder with the default prompt templates.
func NewBuilder() (Builder, error) {
	fr, err := template.New("first-round").Parse(FirstRoundTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse first-round template: %w", err)
	}
	rs, err := template.New("resume").Parse(ResumeTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse resume template: %w", err)
	}
	return &builder{firstRound: fr, resume: rs}, nil
}

func (b *builder) BuildFirstRound(input FirstRoundInput) (string, error) {
	var buf bytes.Buffer
	if err := b.firstRound.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("execute first-round template: %w", err)
	}
	return buf.String(), nil
}

func (b *builder) BuildResume(input ResumeInput) (string, error) {
	var buf bytes.Buffer
	if err := b.resume.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("execute resume template: %w", err)
	}
	return buf.String(), nil
}

func (b *builder) FormatFindingsForPrompt(findings []session.Finding) string {
	// TODO: implement — format findings as human-readable text for prompt
	return ""
}
```

**Step 5: Create internal/parser/extract.go**

```go
package parser

import (
	"fmt"
	"regexp"
	"strings"
)

var uuidRegex = regexp.MustCompile(
	`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`,
)

// ExtractJSON attempts to extract clean JSON from raw codex output.
// If the output is wrapped in code fences, they are stripped.
func ExtractJSON(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("empty output")
	}

	// Try direct parse first (--output-schema should produce clean JSON)
	if strings.HasPrefix(trimmed, "{") {
		return trimmed, nil
	}

	// Fallback: strip markdown code fences
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		var jsonLines []string
		inside := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inside = !inside
				continue
			}
			if inside {
				jsonLines = append(jsonLines, line)
			}
		}
		result := strings.Join(jsonLines, "\n")
		if strings.TrimSpace(result) != "" {
			return result, nil
		}
	}

	return "", fmt.Errorf("could not extract JSON from output")
}

// ExtractCodexSessionID extracts a UUID session ID from codex stderr output.
func ExtractCodexSessionID(stderr string) (string, error) {
	match := uuidRegex.FindString(stderr)
	if match == "" {
		return "", fmt.Errorf("no session ID found in codex stderr")
	}
	return match, nil
}
```

**Step 6: Create internal/parser/parser.go**

```go
package parser

import (
	"encoding/json"
	"fmt"

	"github.com/davidleitw/xreview/internal/session"
)

// Parser parses codex stdout into structured findings.
type Parser interface {
	Parse(stdout string) (*session.CodexResponse, error)
}

type parser struct{}

// NewParser creates a Parser.
func NewParser() Parser {
	return &parser{}
}

func (p *parser) Parse(stdout string) (*session.CodexResponse, error) {
	cleaned, err := ExtractJSON(stdout)
	if err != nil {
		return nil, fmt.Errorf("extract JSON: %w", err)
	}

	var resp session.CodexResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal codex response: %w", err)
	}

	return &resp, nil
}
```

**Step 7: Create internal/formatter/error.go**

```go
package formatter

// Error codes returned by xreview.
const (
	ErrCodexNotFound         = "CODEX_NOT_FOUND"
	ErrCodexNotAuthenticated = "CODEX_NOT_AUTHENTICATED"
	ErrCodexUnresponsive     = "CODEX_UNRESPONSIVE"
	ErrCodexTimeout          = "CODEX_TIMEOUT"
	ErrCodexError            = "CODEX_ERROR"
	ErrParseFailure          = "PARSE_FAILURE"
	ErrSessionNotFound       = "SESSION_NOT_FOUND"
	ErrNoTargets             = "NO_TARGETS"
	ErrInvalidFlags          = "INVALID_FLAGS"
	ErrFileNotFound          = "FILE_NOT_FOUND"
	ErrNotGitRepo            = "NOT_GIT_REPO"
	ErrUpdateFailed          = "UPDATE_FAILED"
	ErrVersionCheckFailed    = "VERSION_CHECK_FAILED"
)

// FormatError produces an XML error response.
func FormatError(action, code, message string) string {
	// TODO: implement — produce <xreview-result status="error" action="...">
	return ""
}
```

**Step 8: Create internal/formatter/xml.go**

```go
package formatter

import "github.com/davidleitw/xreview/internal/session"

// Check represents a preflight check result.
type Check struct {
	Name   string
	Passed bool
	Detail string
}

// FormatReviewResult produces XML output for a review/verify round.
func FormatReviewResult(sessionID string, round int, action string, findings []session.Finding, summary session.FindingSummary) string {
	// TODO: implement
	return ""
}

// FormatPreflightResult produces XML output for a preflight check.
func FormatPreflightResult(checks []Check) string {
	// TODO: implement
	return ""
}

// FormatVersionResult produces XML output for a version check.
func FormatVersionResult(current, latest string, outdated bool) string {
	// TODO: implement
	return ""
}

// FormatReportResult produces XML output for a report generation.
func FormatReportResult(sessionID, path string, summary session.FindingSummary) string {
	// TODO: implement
	return ""
}

// FormatCleanResult produces XML output for a session cleanup.
func FormatCleanResult(sessionID string) string {
	// TODO: implement
	return ""
}
```

**Step 9: Verify compile**

Run: `go build ./internal/...`
Expected: no errors

**Step 10: Commit**

```bash
git add internal/collector/ internal/prompt/ internal/parser/ internal/formatter/
git commit -m "feat: add collector, prompt, parser, formatter packages"
```

---

### Task 5: Service Packages (codex, session manager, reviewer)

**Files:**
- Create: `internal/codex/runner.go`
- Create: `internal/codex/resume.go`
- Create: `internal/session/manager.go`
- Create: `internal/reviewer/reviewer.go`
- Create: `internal/reviewer/single.go`

**Step 1: Create internal/codex/runner.go**

```go
package codex

import (
	"context"
	"fmt"
	"time"
)

// ExecRequest holds parameters for a codex exec call.
type ExecRequest struct {
	Model           string
	Prompt          string
	SchemaPath      string
	Timeout         time.Duration
	ResumeSessionID string // empty for new sessions
}

// ExecResult holds the output from a codex exec call.
type ExecResult struct {
	Stdout         string
	Stderr         string
	CodexSessionID string
	DurationMs     int64
}

// Runner executes codex as a subprocess.
type Runner interface {
	Exec(ctx context.Context, req ExecRequest) (*ExecResult, error)
}

type runner struct{}

// NewRunner creates a Runner.
func NewRunner() Runner {
	return &runner{}
}

func (r *runner) Exec(ctx context.Context, req ExecRequest) (*ExecResult, error) {
	// TODO: implement — spawn codex exec subprocess, capture stdout/stderr
	return nil, fmt.Errorf("codex runner: not implemented")
}
```

**Step 2: Create internal/codex/resume.go**

```go
package codex

import "github.com/davidleitw/xreview/internal/session"

// ShouldResume determines whether to resume an existing codex session
// or start a fresh one.
func ShouldResume(sess *session.Session, fullRescan bool) bool {
	if fullRescan {
		return false
	}
	return sess.CodexSessionID != ""
}
```

**Step 3: Create internal/session/manager.go**

```go
package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/davidleitw/xreview/internal/config"
	"github.com/davidleitw/xreview/internal/version"
)

// Manager handles session CRUD operations.
type Manager interface {
	Create(targets []string, targetMode, context string, cfg *config.Config) (*Session, error)
	Load(sessionID string) (*Session, error)
	Update(sess *Session) error
	Delete(sessionID string) error
	List() ([]string, error)
}

type manager struct {
	sessionsDir string
}

// NewManager creates a Manager that stores sessions in the given workdir.
func NewManager(workdir string) Manager {
	return &manager{
		sessionsDir: config.SessionsDir(workdir),
	}
}

func (m *manager) Create(targets []string, targetMode, ctx string, cfg *config.Config) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	sess := &Session{
		SessionID:      id,
		XReviewVersion: version.Version,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		Status:         StatusInitialized,
		Round:          0,
		CodexModel:     cfg.CodexModel,
		Context:        ctx,
		Targets:        targets,
		TargetMode:     targetMode,
		Findings:       []Finding{},
	}

	dir := filepath.Join(m.sessionsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	if err := m.write(sess); err != nil {
		return nil, err
	}

	return sess, nil
}

func (m *manager) Load(sessionID string) (*Session, error) {
	path := filepath.Join(m.sessionsDir, sessionID, "session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %q not found", sessionID)
		}
		return nil, err
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parse session.json: %w", err)
	}
	return &sess, nil
}

func (m *manager) Update(sess *Session) error {
	sess.UpdatedAt = time.Now().UTC()
	return m.write(sess)
}

func (m *manager) Delete(sessionID string) error {
	dir := filepath.Join(m.sessionsDir, sessionID)
	return os.RemoveAll(dir)
}

func (m *manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

func (m *manager) write(sess *Session) error {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := filepath.Join(m.sessionsDir, sess.SessionID, "session.json")
	return os.WriteFile(path, data, 0o644)
}

func generateSessionID() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	date := time.Now().Format("20060102")
	return fmt.Sprintf("xr-%s-%s", date, hex.EncodeToString(b)), nil
}
```

**Step 4: Create internal/reviewer/reviewer.go**

```go
package reviewer

import (
	"context"

	"github.com/davidleitw/xreview/internal/session"
)

// ReviewRequest holds parameters for starting a new review.
type ReviewRequest struct {
	Targets    []string
	TargetMode string // "files" or "git-uncommitted"
	Context    string
	Timeout    int
}

// ReviewResult holds the output of a review round.
type ReviewResult struct {
	SessionID string
	Round     int
	Verdict   string
	Findings  []session.Finding
	Summary   session.FindingSummary
}

// VerifyRequest holds parameters for a follow-up verification round.
type VerifyRequest struct {
	SessionID  string
	Message    string
	FullRescan bool
	Timeout    int
}

// VerifyResult holds the output of a verification round.
type VerifyResult struct {
	SessionID string
	Round     int
	Verdict   string
	Findings  []session.Finding
	Summary   session.FindingSummary
}

// Reviewer abstracts single vs. multi-agent review.
// Day 1: SingleReviewer (one codex call).
// Future: MultiReviewer (parallel codex calls with aggregation).
type Reviewer interface {
	Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error)
	Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error)
}
```

**Step 5: Create internal/reviewer/single.go**

```go
package reviewer

import (
	"context"
	"fmt"

	"github.com/davidleitw/xreview/internal/codex"
	"github.com/davidleitw/xreview/internal/collector"
	"github.com/davidleitw/xreview/internal/parser"
	"github.com/davidleitw/xreview/internal/prompt"
	"github.com/davidleitw/xreview/internal/session"
)

// SingleReviewer uses a single codex call for review.
type SingleReviewer struct {
	runner    codex.Runner
	builder   prompt.Builder
	parser    parser.Parser
	sessions  session.Manager
	collector collector.Collector
}

// NewSingleReviewer creates a SingleReviewer with the given dependencies.
func NewSingleReviewer(
	runner codex.Runner,
	builder prompt.Builder,
	parser parser.Parser,
	sessions session.Manager,
	collector collector.Collector,
) *SingleReviewer {
	return &SingleReviewer{
		runner:    runner,
		builder:   builder,
		parser:    parser,
		sessions:  sessions,
		collector: collector,
	}
}

func (r *SingleReviewer) Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	// TODO: implement
	return nil, fmt.Errorf("review: not implemented")
}

func (r *SingleReviewer) Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	// TODO: implement
	return nil, fmt.Errorf("verify: not implemented")
}
```

**Step 6: Verify compile**

Run: `go build ./internal/...`
Expected: no errors, no import cycles

**Step 7: Commit**

```bash
git add internal/codex/ internal/session/manager.go internal/reviewer/
git commit -m "feat: add codex runner, session manager, reviewer interface"
```

---

### Task 6: CLI Layer (cobra root + 6 subcommands)

**Files:**
- Create: `cmd/xreview/main.go`
- Create: `cmd/xreview/cmd_version.go`
- Create: `cmd/xreview/cmd_selfupdate.go`
- Create: `cmd/xreview/cmd_preflight.go`
- Create: `cmd/xreview/cmd_review.go`
- Create: `cmd/xreview/cmd_report.go`
- Create: `cmd/xreview/cmd_clean.go`

**Step 1: Create cmd/xreview/main.go**

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

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xreview",
		Short: "Agent-Native Code Review Engine",
		Long:  "xreview orchestrates code review between Claude Code and Codex.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&flagWorkdir, "workdir", ".", "Working directory (default: current directory)")
	cmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Print debug information to stderr")
	cmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output raw JSON instead of XML")

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newSelfUpdateCmd())
	cmd.AddCommand(newPreflightCmd())
	cmd.AddCommand(newReviewCmd())
	cmd.AddCommand(newReportCmd())
	cmd.AddCommand(newCleanCmd())

	return cmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 2: Create cmd/xreview/cmd_version.go**

```go
package main

import (
	"fmt"

	"github.com/davidleitw/xreview/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show xreview version",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: implement full version check with GitHub API
			fmt.Println(version.Version)
			return nil
		},
	}
}
```

**Step 3: Create cmd/xreview/cmd_selfupdate.go**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSelfUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "self-update",
		Short: "Update xreview to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("self-update: not implemented")
		},
	}
}
```

**Step 4: Create cmd/xreview/cmd_preflight.go**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPreflightCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preflight",
		Short: "Verify codex environment is ready",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("preflight: not implemented")
		},
	}
}
```

**Step 5: Create cmd/xreview/cmd_review.go**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newReviewCmd() *cobra.Command {
	var (
		files          string
		gitUncommitted bool
		sessionID      string
		message        string
		fullRescan     bool
		timeout        int
	)

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Run code review or continue existing session",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			hasFiles := files != ""
			hasGit := gitUncommitted
			hasSession := sessionID != ""

			// --files and --git-uncommitted are mutually exclusive
			if hasFiles && hasGit {
				return fmt.Errorf("--files and --git-uncommitted are mutually exclusive")
			}

			// New review requires --files or --git-uncommitted
			if !hasSession && !hasFiles && !hasGit {
				return fmt.Errorf("new review requires --files or --git-uncommitted")
			}

			// --files/--git-uncommitted cannot combine with --session
			if hasSession && (hasFiles || hasGit) {
				return fmt.Errorf("--files/--git-uncommitted cannot be used with --session")
			}

			// --message and --full-rescan require --session
			if !hasSession && message != "" {
				return fmt.Errorf("--message requires --session")
			}
			if !hasSession && fullRescan {
				return fmt.Errorf("--full-rescan requires --session")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("review: not implemented")
		},
	}

	cmd.Flags().StringVar(&files, "files", "", "Comma-separated file or directory paths to review")
	cmd.Flags().BoolVar(&gitUncommitted, "git-uncommitted", false, "Review all uncommitted changes")
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID for continuing a review")
	cmd.Flags().StringVar(&message, "message", "", "Message describing fixes or dismissals")
	cmd.Flags().BoolVar(&fullRescan, "full-rescan", false, "Start fresh codex session for rescan")
	cmd.Flags().IntVar(&timeout, "timeout", 180, "Timeout in seconds for codex response")

	return cmd
}
```

**Step 6: Create cmd/xreview/cmd_report.go**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newReportCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate review report",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("report: not implemented")
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to generate report for")

	return cmd
}
```

**Step 7: Create cmd/xreview/cmd_clean.go**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Delete session data",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("clean: not implemented")
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to delete")

	return cmd
}
```

**Step 8: Build and verify**

Run: `make build`
Expected: `bin/xreview` binary created

Run: `./bin/xreview --help`
Expected: shows root help with all 6 subcommands

Run: `./bin/xreview version`
Expected: prints `dev`

Run: `./bin/xreview review --help`
Expected: shows all review flags

Run: `./bin/xreview review --files a --session b`
Expected: error about mutually exclusive flags

**Step 9: Commit**

```bash
git add cmd/
git commit -m "feat: add CLI layer with cobra root and 6 subcommands"
```

---

### Task 7: Claude Code Skill Files

**Files:**
- Create: `.claude/skills/xreview/SKILL.md`
- Create: `.claude/skills/xreview/reference.md`

**Step 1: Create .claude/skills/xreview/SKILL.md**

Content is the complete SKILL.md from design spec Section 8 (lines 1315-1452). Copy verbatim including the YAML frontmatter with `name: xreview`, `allowed-tools`, and the full Step 0-8 workflow with three-party consensus.

**Step 2: Create .claude/skills/xreview/reference.md**

Content is the complete reference.md from design spec Section 8 (lines 1456-1500). Copy verbatim including all XML elements and error codes.

**Step 3: Commit**

```bash
git add .claude/
git commit -m "feat: add Claude Code skill for xreview review workflow"
```

---

### Task 8: Final Verification

**Step 1: Run go vet**

Run: `go vet ./...`
Expected: no issues

**Step 2: Run full build**

Run: `go build ./...`
Expected: all packages compile

**Step 3: Test binary**

Run: `make build && ./bin/xreview --help`
Expected: root help with clean, preflight, report, review, self-update, version

Run: `./bin/xreview version`
Expected: `dev`

Run: `./bin/xreview preflight`
Expected: `preflight: not implemented`

Run: `./bin/xreview review --files foo --session bar`
Expected: `--files/--git-uncommitted cannot be used with --session`

Run: `./bin/xreview review`
Expected: `new review requires --files or --git-uncommitted`

Run: `./bin/xreview report`
Expected: `--session is required`

**Step 4: Final commit (if any cleanup needed)**

```bash
git add -A
git commit -m "chore: project scaffold complete"
```
