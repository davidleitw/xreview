# Plan 2: Session Manager

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement session CRUD — create sessions, read/update state, manage findings, write rounds, and handle finding comparison for full-rescan.

**Architecture:** `internal/session/` package. Manager handles filesystem operations on `.xreview/sessions/<id>/`. Findings module handles state transitions and summary computation. Comparison module handles diff for full-rescan.

**Tech Stack:** Go stdlib (`encoding/json`, `os`, `path/filepath`, `crypto/rand`, `fmt`, `time`)

**Depends on:** Plan 1 (types.go must exist)

---

## Chunk 1: Session Manager — Create, Read, Update, List

### File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `internal/session/manager.go` | Session CRUD: Create, Load, Save, List, Delete, directory layout |
| Create | `internal/session/manager_test.go` | All manager operations |

---

### Task 2.1: Session Manager — Create + Load

**Files:**
- Create: `internal/session/manager.go`
- Test: `internal/session/manager_test.go`

- [ ] **Step 1: Write failing tests for Create and Load**

Create `internal/session/manager_test.go`:

```go
package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManager_Create(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	sess, err := mgr.Create([]string{"src/auth.go"}, "files", "test context", 180)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Validate session ID format: xr-YYYYMMDD-<6hex>
	if !strings.HasPrefix(sess.SessionID, "xr-") {
		t.Errorf("session ID should start with 'xr-', got %q", sess.SessionID)
	}
	parts := strings.Split(sess.SessionID, "-")
	if len(parts) != 3 {
		t.Errorf("session ID should have 3 parts, got %d: %q", len(parts), sess.SessionID)
	}
	if len(parts[2]) != 6 {
		t.Errorf("random suffix should be 6 chars, got %d", len(parts[2]))
	}

	if sess.Status != StatusInitialized {
		t.Errorf("status: got %q, want %q", sess.Status, StatusInitialized)
	}
	if sess.CurrentRound != 0 {
		t.Errorf("current_round: got %d, want 0", sess.CurrentRound)
	}

	// Verify directory was created
	sessionDir := filepath.Join(dir, ".xreview", "sessions", sess.SessionID)
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		t.Errorf("session directory not created: %s", sessionDir)
	}
	// Verify subdirectories
	for _, sub := range []string{"rounds", "raw"} {
		p := filepath.Join(sessionDir, sub)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("subdirectory not created: %s", p)
		}
	}

	// Verify session.json was written
	if _, err := os.Stat(filepath.Join(sessionDir, "session.json")); os.IsNotExist(err) {
		t.Error("session.json not created")
	}
}

func TestManager_Load(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	created, err := mgr.Create([]string{"src/auth.go"}, "files", "test", 180)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	loaded, err := mgr.Load(created.SessionID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.SessionID != created.SessionID {
		t.Errorf("session_id: got %q, want %q", loaded.SessionID, created.SessionID)
	}
	if loaded.Context != "test" {
		t.Errorf("context: got %q, want %q", loaded.Context, "test")
	}
}

func TestManager_Load_NotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	_, err := mgr.Load("xr-99999999-ffffff")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v -run TestManager`
Expected: FAIL — NewManager not defined

- [ ] **Step 3: Write manager.go**

Create `internal/session/manager.go`:

```go
package session

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/davidleitw/xreview/internal/version"
)

// Manager handles session CRUD on the filesystem.
type Manager struct {
	workdir string
}

// NewManager creates a Manager rooted at the given working directory.
func NewManager(workdir string) *Manager {
	return &Manager{workdir: workdir}
}

// sessionsDir returns the path to .xreview/sessions/.
func (m *Manager) sessionsDir() string {
	return filepath.Join(m.workdir, ".xreview", "sessions")
}

// sessionDir returns the path to a specific session directory.
func (m *Manager) sessionDir(id string) string {
	return filepath.Join(m.sessionsDir(), id)
}

// generateID creates a session ID: xr-YYYYMMDD-<6hex>.
func generateID() string {
	b := make([]byte, 3)
	rand.Read(b)
	return fmt.Sprintf("xr-%s-%x", time.Now().Format("20060102"), b)
}

// Create initializes a new session on disk.
func (m *Manager) Create(targets []string, targetMode, context string, timeout int) (*Session, error) {
	id := generateID()
	now := time.Now().UTC().Format(time.RFC3339)

	sess := &Session{
		SessionID:      id,
		XReviewVersion: version.Version,
		CreatedAt:      now,
		UpdatedAt:      now,
		Status:         StatusInitialized,
		CurrentRound:   0,
		Context:        context,
		Targets:        targets,
		TargetMode:     targetMode,
		Config:         SessionConfig{Timeout: timeout},
	}

	dir := m.sessionDir(id)
	for _, sub := range []string{"rounds", "raw"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return nil, fmt.Errorf("create session dirs: %w", err)
		}
	}

	if err := m.saveSession(sess); err != nil {
		return nil, err
	}

	return sess, nil
}

// Load reads a session from disk.
func (m *Manager) Load(id string) (*Session, error) {
	path := filepath.Join(m.sessionDir(id), "session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load session %s: %w", id, err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parse session %s: %w", id, err)
	}
	return &sess, nil
}

// Save writes a session to disk (updates updated_at).
func (m *Manager) Save(sess *Session) error {
	sess.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return m.saveSession(sess)
}

func (m *Manager) saveSession(sess *Session) error {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	path := filepath.Join(m.sessionDir(sess.SessionID), "session.json")
	return os.WriteFile(path, data, 0644)
}

// List returns all session IDs in .xreview/sessions/.
func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.sessionsDir())
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

// Delete removes a session directory.
func (m *Manager) Delete(id string) error {
	dir := m.sessionDir(id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("session %s not found", id)
	}
	return os.RemoveAll(dir)
}

// SaveRound writes a round-NNN.json file.
func (m *Manager) SaveRound(id string, round *Round) error {
	filename := fmt.Sprintf("round-%03d.json", round.RoundNum)
	path := filepath.Join(m.sessionDir(id), "rounds", filename)

	data, err := json.MarshalIndent(round, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal round: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// SaveRawOutput writes raw codex stdout/stderr for a round.
func (m *Manager) SaveRawOutput(id string, roundNum int, stdout, stderr string) error {
	rawDir := filepath.Join(m.sessionDir(id), "raw")
	prefix := fmt.Sprintf("round-%03d-codex", roundNum)

	if err := os.WriteFile(filepath.Join(rawDir, prefix+"-stdout.txt"), []byte(stdout), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(rawDir, prefix+"-stderr.txt"), []byte(stderr), 0644)
}

// SaveFindings writes findings.json for a session.
func (m *Manager) SaveFindings(id string, ff *FindingsFile) error {
	data, err := json.MarshalIndent(ff, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal findings: %w", err)
	}
	path := filepath.Join(m.sessionDir(id), "findings.json")
	return os.WriteFile(path, data, 0644)
}

// LoadFindings reads findings.json for a session.
func (m *Manager) LoadFindings(id string) (*FindingsFile, error) {
	path := filepath.Join(m.sessionDir(id), "findings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FindingsFile{}, nil
		}
		return nil, fmt.Errorf("load findings %s: %w", id, err)
	}

	var ff FindingsFile
	if err := json.Unmarshal(data, &ff); err != nil {
		return nil, fmt.Errorf("parse findings %s: %w", id, err)
	}
	return &ff, nil
}

// SessionDir returns the absolute path to a session's directory (for external use).
func (m *Manager) SessionDir(id string) string {
	return m.sessionDir(id)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v -run TestManager`
Expected: All 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/session/manager.go internal/session/manager_test.go
git commit -m "feat: add session manager with create, load, save, list, delete"
```

---

### Task 2.2: Session Manager — List, Delete, SaveRound, SaveFindings

**Files:**
- Modify: `internal/session/manager_test.go` (add tests)

- [ ] **Step 1: Add tests for List, Delete, rounds, findings, raw output**

Append to `internal/session/manager_test.go`:

```go
func TestManager_List(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	// Empty list
	ids, err := mgr.List()
	if err != nil {
		t.Fatalf("List (empty): %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(ids))
	}

	// Create two sessions
	s1, _ := mgr.Create([]string{"a.go"}, "files", "", 180)
	s2, _ := mgr.Create([]string{"b.go"}, "files", "", 180)

	ids, err = mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(ids))
	}

	// Verify both IDs are present
	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found[s1.SessionID] || !found[s2.SessionID] {
		t.Errorf("missing session IDs: got %v", ids)
	}
}

func TestManager_Delete(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	sess, _ := mgr.Create([]string{"a.go"}, "files", "", 180)

	if err := mgr.Delete(sess.SessionID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := mgr.Load(sess.SessionID)
	if err == nil {
		t.Fatal("expected error loading deleted session")
	}
}

func TestManager_Delete_NotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	err := mgr.Delete("xr-99999999-ffffff")
	if err == nil {
		t.Fatal("expected error deleting nonexistent session")
	}
}

func TestManager_SaveRound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	sess, _ := mgr.Create([]string{"a.go"}, "files", "", 180)

	round := &Round{
		RoundNum:  1,
		Timestamp: "2026-03-10T14:30:05Z",
		Action:    "review",
	}

	if err := mgr.SaveRound(sess.SessionID, round); err != nil {
		t.Fatalf("SaveRound: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, ".xreview", "sessions", sess.SessionID, "rounds", "round-001.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("round-001.json not created")
	}
}

func TestManager_SaveAndLoadFindings(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	sess, _ := mgr.Create([]string{"a.go"}, "files", "", 180)

	ff := &FindingsFile{
		LastUpdatedRound: 1,
		Findings: []Finding{
			{ID: "F001", Severity: "high", Status: FindingOpen},
		},
		Summary: FindingSummary{Total: 1, Open: 1},
	}

	if err := mgr.SaveFindings(sess.SessionID, ff); err != nil {
		t.Fatalf("SaveFindings: %v", err)
	}

	loaded, err := mgr.LoadFindings(sess.SessionID)
	if err != nil {
		t.Fatalf("LoadFindings: %v", err)
	}

	if loaded.LastUpdatedRound != 1 {
		t.Errorf("last_updated_round: got %d, want 1", loaded.LastUpdatedRound)
	}
	if len(loaded.Findings) != 1 {
		t.Errorf("findings len: got %d, want 1", len(loaded.Findings))
	}
}

func TestManager_SaveRawOutput(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	sess, _ := mgr.Create([]string{"a.go"}, "files", "", 180)

	if err := mgr.SaveRawOutput(sess.SessionID, 1, "stdout content", "stderr content"); err != nil {
		t.Fatalf("SaveRawOutput: %v", err)
	}

	rawDir := filepath.Join(dir, ".xreview", "sessions", sess.SessionID, "raw")

	stdoutData, _ := os.ReadFile(filepath.Join(rawDir, "round-001-codex-stdout.txt"))
	if string(stdoutData) != "stdout content" {
		t.Errorf("stdout: got %q", string(stdoutData))
	}

	stderrData, _ := os.ReadFile(filepath.Join(rawDir, "round-001-codex-stderr.txt"))
	if string(stderrData) != "stderr content" {
		t.Errorf("stderr: got %q", string(stderrData))
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v -run "TestManager_"`
Expected: All tests PASS (manager.go already has these methods)

- [ ] **Step 3: Commit**

```bash
git add internal/session/manager_test.go
git commit -m "test: add comprehensive session manager tests"
```

---

## Chunk 2: Findings State Management + Comparison

### File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `internal/session/findings.go` | Finding state transitions, summary computation, ID assignment |
| Create | `internal/session/findings_test.go` | State transition tests |
| Create | `internal/session/comparison.go` | Full-rescan finding diff (recurring/new/resolved) |
| Create | `internal/session/comparison_test.go` | Comparison tests |

---

### Task 2.3: Findings State Management

**Files:**
- Create: `internal/session/findings.go`
- Test: `internal/session/findings_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/session/findings_test.go`:

```go
package session

import (
	"testing"
)

func TestAssignFindingIDs(t *testing.T) {
	findings := []CodexFinding{
		{Severity: "high", Description: "issue 1"},
		{Severity: "medium", Description: "issue 2"},
		{Severity: "low", Description: "issue 3"},
	}

	result := AssignFindingIDs(findings, 1)

	if len(result) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(result))
	}
	if result[0].ID != "F001" {
		t.Errorf("first ID: got %q, want F001", result[0].ID)
	}
	if result[2].ID != "F003" {
		t.Errorf("third ID: got %q, want F003", result[2].ID)
	}
	if result[0].FirstSeenRound != 1 {
		t.Errorf("first_seen_round: got %d, want 1", result[0].FirstSeenRound)
	}
	if result[0].Status != FindingOpen {
		t.Errorf("status: got %q, want %q", result[0].Status, FindingOpen)
	}
}

func TestComputeSummary(t *testing.T) {
	findings := []Finding{
		{Status: FindingOpen},
		{Status: FindingOpen},
		{Status: FindingFixed},
		{Status: FindingDismissed},
	}

	s := ComputeSummary(findings)

	if s.Total != 4 {
		t.Errorf("total: got %d, want 4", s.Total)
	}
	if s.Open != 2 {
		t.Errorf("open: got %d, want 2", s.Open)
	}
	if s.Fixed != 1 {
		t.Errorf("fixed: got %d, want 1", s.Fixed)
	}
	if s.Dismissed != 1 {
		t.Errorf("dismissed: got %d, want 1", s.Dismissed)
	}
}

func TestMergeFindings_ResumeRound(t *testing.T) {
	existing := []Finding{
		{ID: "F001", Severity: "high", Status: FindingOpen, FirstSeenRound: 1},
		{ID: "F002", Severity: "medium", Status: FindingOpen, FirstSeenRound: 1},
	}

	codexFindings := []CodexFinding{
		{ID: "F001", Severity: "high", Status: "fixed", VerificationNote: "fix verified"},
		{ID: "F002", Severity: "medium", Status: "dismissed", VerificationNote: "reasonable"},
		{Severity: "medium", Description: "new issue", Status: "open"},
	}

	result := MergeFindings(existing, codexFindings, 2)

	if len(result) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(result))
	}

	// F001 should be fixed
	if result[0].Status != FindingFixed {
		t.Errorf("F001 status: got %q, want fixed", result[0].Status)
	}
	if result[0].VerificationNote != "fix verified" {
		t.Errorf("F001 verification: got %q", result[0].VerificationNote)
	}

	// F002 should be dismissed
	if result[1].Status != FindingDismissed {
		t.Errorf("F002 status: got %q, want dismissed", result[1].Status)
	}

	// New finding should get next ID
	if result[2].ID != "F003" {
		t.Errorf("new finding ID: got %q, want F003", result[2].ID)
	}
	if result[2].FirstSeenRound != 2 {
		t.Errorf("new finding first_seen_round: got %d, want 2", result[2].FirstSeenRound)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v -run "TestAssign|TestCompute|TestMerge"`
Expected: FAIL — functions not defined

- [ ] **Step 3: Write findings.go**

Create `internal/session/findings.go`:

```go
package session

import "fmt"

// AssignFindingIDs converts codex findings to internal findings with sequential IDs.
// Used for first-round reviews where no previous findings exist.
func AssignFindingIDs(codexFindings []CodexFinding, round int) []Finding {
	findings := make([]Finding, 0, len(codexFindings))
	for i, cf := range codexFindings {
		f := Finding{
			ID:               fmt.Sprintf("F%03d", i+1),
			Severity:         cf.Severity,
			Category:         cf.Category,
			Status:           FindingOpen,
			File:             cf.File,
			Line:             cf.Line,
			Description:      cf.Description,
			Suggestion:       cf.Suggestion,
			CodeSnippet:      cf.CodeSnippet,
			FirstSeenRound:   round,
			LastUpdatedRound: round,
			History: []FindingHistoryEntry{
				{Round: round, Status: FindingOpen, Note: "initial finding"},
			},
		}
		findings = append(findings, f)
	}
	return findings
}

// ComputeSummary calculates finding counts by status.
func ComputeSummary(findings []Finding) FindingSummary {
	s := FindingSummary{Total: len(findings)}
	for _, f := range findings {
		switch f.Status {
		case FindingOpen, FindingReopened:
			s.Open++
		case FindingFixed:
			s.Fixed++
		case FindingDismissed:
			s.Dismissed++
		}
	}
	return s
}

// MergeFindings merges codex resume/verify output with existing findings.
// Codex findings with an ID update existing findings. Findings without an ID are new.
func MergeFindings(existing []Finding, codexFindings []CodexFinding, round int) []Finding {
	byID := make(map[string]*Finding)
	for i := range existing {
		byID[existing[i].ID] = &existing[i]
	}

	maxID := len(existing)
	var newFindings []Finding

	for _, cf := range codexFindings {
		if cf.ID != "" {
			if f, ok := byID[cf.ID]; ok {
				f.Status = cf.Status
				f.VerificationNote = cf.VerificationNote
				f.LastUpdatedRound = round
				f.History = append(f.History, FindingHistoryEntry{
					Round:  round,
					Status: cf.Status,
					Note:   cf.VerificationNote,
				})
			}
		} else {
			maxID++
			newFindings = append(newFindings, Finding{
				ID:               fmt.Sprintf("F%03d", maxID),
				Severity:         cf.Severity,
				Category:         cf.Category,
				Status:           FindingOpen,
				File:             cf.File,
				Line:             cf.Line,
				Description:      cf.Description,
				Suggestion:       cf.Suggestion,
				CodeSnippet:      cf.CodeSnippet,
				FirstSeenRound:   round,
				LastUpdatedRound: round,
				History: []FindingHistoryEntry{
					{Round: round, Status: FindingOpen, Note: "new finding in round " + fmt.Sprintf("%d", round)},
				},
			})
		}
	}

	result := make([]Finding, len(existing))
	copy(result, existing)
	result = append(result, newFindings...)
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v -run "TestAssign|TestCompute|TestMerge"`
Expected: All 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/session/findings.go internal/session/findings_test.go
git commit -m "feat: add finding ID assignment, summary computation, and merge logic"
```

---

### Task 2.4: Finding Comparison (Full Rescan)

**Files:**
- Create: `internal/session/comparison.go`
- Test: `internal/session/comparison_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/session/comparison_test.go`:

```go
package session

import (
	"testing"
)

func TestCompareFindings(t *testing.T) {
	previous := []Finding{
		{ID: "F001", Severity: "high", File: "auth.go", Line: 42, Description: "JWT issue"},
		{ID: "F002", Severity: "medium", File: "mid.go", Line: 15, Description: "Error wrapping"},
		{ID: "F003", Severity: "low", File: "utils.go", Line: 88, Description: "Nil pointer"},
	}

	current := []CodexFinding{
		{Severity: "low", File: "utils.go", Line: 88, Description: "Nil pointer still present"},
		{Severity: "high", File: "auth.go", Line: 60, Description: "Hardcoded secret"},
	}

	result := CompareFindings(previous, current, 3)

	// F003 should be recurring (same file+line range)
	found := false
	for _, f := range result.Findings {
		if f.ID == "F003" && f.Comparison == "recurring" {
			found = true
			break
		}
	}
	if !found {
		t.Error("F003 should be marked as recurring")
	}

	// New finding should exist with comparison="new"
	hasNew := false
	for _, f := range result.Findings {
		if f.Comparison == "new" && f.File == "auth.go" && f.Line == 60 {
			hasNew = true
			break
		}
	}
	if !hasNew {
		t.Error("expected new finding for auth.go:60")
	}

	// F001 and F002 should be in resolved
	if len(result.Resolved) != 2 {
		t.Errorf("expected 2 resolved, got %d", len(result.Resolved))
	}
}

func TestCompareFindings_AllResolved(t *testing.T) {
	previous := []Finding{
		{ID: "F001", Severity: "high", File: "auth.go", Line: 42},
	}

	current := []CodexFinding{} // no findings in rescan

	result := CompareFindings(previous, current, 2)

	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
	if len(result.Resolved) != 1 {
		t.Errorf("expected 1 resolved, got %d", len(result.Resolved))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v -run TestCompareFindings`
Expected: FAIL — CompareFindings not defined

- [ ] **Step 3: Write comparison.go**

Create `internal/session/comparison.go`:

```go
package session

import "fmt"

// ComparisonResult holds the result of comparing previous and current findings.
type ComparisonResult struct {
	Findings []Finding
	Resolved []ResolvedFinding
}

// ResolvedFinding represents a previously-found issue no longer detected.
type ResolvedFinding struct {
	ID   string
	Note string
}

// CompareFindings compares previous findings against a fresh rescan.
// Returns findings annotated with comparison metadata and a list of resolved findings.
func CompareFindings(previous []Finding, current []CodexFinding, round int) ComparisonResult {
	matched := make(map[string]bool)
	var findings []Finding
	nextID := len(previous) + 1

	for _, cf := range current {
		matchedID := matchPrevious(previous, cf)
		if matchedID != "" {
			matched[matchedID] = true
			// Find the original finding and update it
			for _, pf := range previous {
				if pf.ID == matchedID {
					f := pf
					f.Status = FindingOpen
					f.Comparison = "recurring"
					f.LastUpdatedRound = round
					f.Description = cf.Description
					if cf.Suggestion != "" {
						f.Suggestion = cf.Suggestion
					}
					f.History = append(f.History, FindingHistoryEntry{
						Round: round, Status: FindingOpen, Note: "still present in rescan",
					})
					findings = append(findings, f)
					break
				}
			}
		} else {
			findings = append(findings, Finding{
				ID:               fmt.Sprintf("F%03d", nextID),
				Severity:         cf.Severity,
				Category:         cf.Category,
				Status:           FindingOpen,
				Comparison:       "new",
				File:             cf.File,
				Line:             cf.Line,
				Description:      cf.Description,
				Suggestion:       cf.Suggestion,
				CodeSnippet:      cf.CodeSnippet,
				FirstSeenRound:   round,
				LastUpdatedRound: round,
				History: []FindingHistoryEntry{
					{Round: round, Status: FindingOpen, Note: "new finding in rescan"},
				},
			})
			nextID++
		}
	}

	var resolved []ResolvedFinding
	for _, pf := range previous {
		if !matched[pf.ID] {
			resolved = append(resolved, ResolvedFinding{
				ID:   pf.ID,
				Note: fmt.Sprintf("%s issue no longer detected in current code.", pf.Category),
			})
		}
	}

	return ComparisonResult{Findings: findings, Resolved: resolved}
}

// matchPrevious tries to find a previous finding that matches a current codex finding.
// Match criteria: same file and line within ±5 lines.
func matchPrevious(previous []Finding, cf CodexFinding) string {
	for _, pf := range previous {
		if pf.File == cf.File && abs(pf.Line-cf.Line) <= 5 {
			return pf.ID
		}
	}
	return ""
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/session/ -v -run TestCompareFindings`
Expected: All 2 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/session/comparison.go internal/session/comparison_test.go
git commit -m "feat: add finding comparison for full-rescan mode"
```
