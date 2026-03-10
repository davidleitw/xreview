# Plan 5: Codex Integration — Runner, Schema, Session ID

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Spawn codex subprocess, manage JSON schema file, extract session ID from stderr, handle resume and timeout.

**Architecture:** `internal/codex/` for process management, `internal/schema/` for JSON schema generation. Runner is the central abstraction that wraps `os/exec` with timeout, schema, and resume support.

**Tech Stack:** Go stdlib (`os/exec`, `context`, `regexp`, `encoding/json`, `os`)

**Depends on:** Plan 1 (types)

---

## Chunk 1: Schema + Runner + Session ID Extraction

### File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `internal/schema/schema.go` | Generate JSON schema file for `--output-schema` |
| Create | `internal/schema/schema_test.go` | Schema generation tests |
| Create | `internal/codex/runner.go` | Spawn codex, capture stdout/stderr, timeout |
| Create | `internal/codex/runner_test.go` | Runner tests (unit, no real codex) |
| Create | `internal/codex/resume.go` | Session ID extraction from stderr |
| Create | `internal/codex/resume_test.go` | Regex extraction tests |

---

### Task 5.1: JSON Schema Generator

**Files:**
- Create: `internal/schema/schema.go`
- Test: `internal/schema/schema_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/schema/schema_test.go`:

```go
package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.json")

	if err := WriteSchema(path); err != nil {
		t.Fatalf("WriteSchema: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	// Verify it's valid JSON
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Check required top-level fields
	if m["type"] != "object" {
		t.Errorf("type: got %v, want object", m["type"])
	}

	props, ok := m["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("missing properties")
	}

	for _, key := range []string{"verdict", "summary", "findings"} {
		if _, ok := props[key]; !ok {
			t.Errorf("missing property: %s", key)
		}
	}

	// Check additionalProperties is false
	if m["additionalProperties"] != false {
		t.Error("additionalProperties should be false")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/schema/ -v`
Expected: FAIL — package not defined

- [ ] **Step 3: Write schema.go**

Create `internal/schema/schema.go`:

```go
package schema

import (
	"encoding/json"
	"os"
)

// WriteSchema writes the codex output JSON schema to the given path.
func WriteSchema(path string) error {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"verdict": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"APPROVED", "REVISE"},
				"description": "Overall review decision",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "Brief summary of review findings",
			},
			"findings": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":                map[string]interface{}{"type": "string"},
						"severity":          map[string]interface{}{"type": "string", "enum": []string{"high", "medium", "low"}},
						"category":          map[string]interface{}{"type": "string", "enum": []string{"security", "logic", "performance", "error-handling"}},
						"file":              map[string]interface{}{"type": "string"},
						"line":              map[string]interface{}{"type": "integer"},
						"description":       map[string]interface{}{"type": "string"},
						"suggestion":        map[string]interface{}{"type": "string"},
						"code_snippet":      map[string]interface{}{"type": "string"},
						"status":            map[string]interface{}{"type": "string", "enum": []string{"open", "fixed", "dismissed", "reopened"}},
						"verification_note": map[string]interface{}{"type": "string"},
					},
					"required":             []string{"id", "severity", "description", "suggestion"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"verdict", "summary", "findings"},
		"additionalProperties": false,
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/schema/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/schema/
git commit -m "feat: add JSON schema generator for codex --output-schema"
```

---

### Task 5.2: Session ID Extraction

**Files:**
- Create: `internal/codex/resume.go`
- Test: `internal/codex/resume_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/codex/resume_test.go`:

```go
package codex

import (
	"testing"
)

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name    string
		stderr  string
		want    string
		wantErr bool
	}{
		{
			name:   "standard UUID in stderr",
			stderr: "Session started: a1b2c3d4-e5f6-7890-abcd-ef1234567890\nProcessing...",
			want:   "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		},
		{
			name:   "UUID surrounded by other text",
			stderr: "[INFO] codex 1.2.3\n[SESSION] 12345678-1234-1234-1234-123456789012 created\n[DONE]",
			want:   "12345678-1234-1234-1234-123456789012",
		},
		{
			name:   "uppercase hex",
			stderr: "Session: AABBCCDD-EEFF-0011-2233-445566778899",
			want:   "AABBCCDD-EEFF-0011-2233-445566778899",
		},
		{
			name:    "no UUID",
			stderr:  "codex started, no session info here",
			wantErr: true,
		},
		{
			name:    "empty stderr",
			stderr:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractSessionID(tt.stderr)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/codex/ -v -run TestExtract`
Expected: FAIL — function not defined

- [ ] **Step 3: Write resume.go**

Create `internal/codex/resume.go`:

```go
package codex

import (
	"fmt"
	"regexp"
)

var uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// ExtractSessionID finds a UUID in codex stderr output.
func ExtractSessionID(stderr string) (string, error) {
	match := uuidRe.FindString(stderr)
	if match == "" {
		return "", fmt.Errorf("no session ID found in codex stderr")
	}
	return match, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/codex/ -v -run TestExtract`
Expected: All 5 sub-tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/codex/resume.go internal/codex/resume_test.go
git commit -m "feat: add codex session ID extraction from stderr"
```

---

### Task 5.3: Codex Runner

**Files:**
- Create: `internal/codex/runner.go`
- Test: `internal/codex/runner_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/codex/runner_test.go`:

```go
package codex

import (
	"context"
	"testing"
)

func TestRunnerOptions(t *testing.T) {
	r := NewRunner("o3", "/tmp/schema.json")

	if r.model != "o3" {
		t.Errorf("model: got %q, want o3", r.model)
	}
	if r.schemaPath != "/tmp/schema.json" {
		t.Errorf("schemaPath: got %q", r.schemaPath)
	}
}

func TestBuildArgs_NewSession(t *testing.T) {
	r := NewRunner("o3", "/tmp/schema.json")

	args := r.buildArgs("review this code", "")

	// Should contain: exec, -m, o3, --skip-git-repo-check, -c, ..., --output-schema, path, prompt
	found := map[string]bool{}
	for _, a := range args {
		found[a] = true
	}

	if !found["exec"] {
		t.Error("missing 'exec' subcommand")
	}
	if !found["o3"] {
		t.Error("missing model")
	}
	if !found["--output-schema"] {
		t.Error("missing --output-schema")
	}
	if !found["--skip-git-repo-check"] {
		t.Error("missing --skip-git-repo-check")
	}

	// Should NOT contain --resume
	if found["--resume"] {
		t.Error("should not have --resume for new session")
	}
}

func TestBuildArgs_ResumeSession(t *testing.T) {
	r := NewRunner("o3", "/tmp/schema.json")

	args := r.buildArgs("verify fixes", "abc-123-def")

	found := map[string]bool{}
	for i, a := range args {
		found[a] = true
		if a == "--resume" && i+1 < len(args) {
			if args[i+1] != "abc-123-def" {
				t.Errorf("--resume value: got %q, want abc-123-def", args[i+1])
			}
		}
	}

	if !found["--resume"] {
		t.Error("missing --resume for resume session")
	}
}

func TestRunner_Exec_CodexNotFound(t *testing.T) {
	r := &Runner{
		model:      "o3",
		schemaPath: "/tmp/schema.json",
		codexBin:   "/nonexistent/codex",
	}

	_, err := r.Exec(context.Background(), "test", "", 10)
	if err == nil {
		t.Fatal("expected error when codex binary not found")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/codex/ -v -run "TestRunner|TestBuild"`
Expected: FAIL — Runner type not defined

- [ ] **Step 3: Write runner.go**

Create `internal/codex/runner.go`:

```go
package codex

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Result holds the output from a codex execution.
type Result struct {
	Stdout    string
	Stderr    string
	SessionID string
	Duration  time.Duration
}

// Runner manages codex subprocess execution.
type Runner struct {
	model      string
	schemaPath string
	codexBin   string
}

// NewRunner creates a Runner with the given model and schema path.
func NewRunner(model, schemaPath string) *Runner {
	return &Runner{
		model:      model,
		schemaPath: schemaPath,
		codexBin:   "codex",
	}
}

// Exec runs codex with the given prompt. If resumeSessionID is non-empty, resumes that session.
// timeout is in seconds.
func (r *Runner) Exec(ctx context.Context, prompt, resumeSessionID string, timeout int) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	args := r.buildArgs(prompt, resumeSessionID)

	cmd := exec.CommandContext(ctx, r.codexBin, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	if ctx.Err() == context.DeadlineExceeded {
		return &Result{Stdout: stdout, Stderr: stderr, Duration: duration},
			fmt.Errorf("codex timed out after %d seconds", timeout)
	}

	if err != nil {
		return &Result{Stdout: stdout, Stderr: stderr, Duration: duration},
			fmt.Errorf("codex exited with error: %w", err)
	}

	// Extract session ID from stderr
	sessionID, _ := ExtractSessionID(stderr)

	return &Result{
		Stdout:    stdout,
		Stderr:    stderr,
		SessionID: sessionID,
		Duration:  duration,
	}, nil
}

// buildArgs constructs the codex CLI arguments.
func (r *Runner) buildArgs(prompt, resumeSessionID string) []string {
	args := []string{"exec"}

	if resumeSessionID != "" {
		args = append(args, "--resume", resumeSessionID)
	}

	args = append(args,
		"-m", r.model,
		"--skip-git-repo-check",
		"-c", "skills.allow_implicit_invocation=false",
		"--output-schema", r.schemaPath,
		prompt,
	)

	return args
}

// LookPath checks if codex binary is available.
func LookPath() (string, error) {
	return exec.LookPath("codex")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/codex/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/codex/runner.go internal/codex/runner_test.go
git commit -m "feat: add codex runner with timeout, resume, and schema support"
```
