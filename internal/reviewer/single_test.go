package reviewer

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/davidleitw/xreview/internal/codex"
	"github.com/davidleitw/xreview/internal/collector"
	"github.com/davidleitw/xreview/internal/config"
	"github.com/davidleitw/xreview/internal/prompt"
	"github.com/davidleitw/xreview/internal/session"
)

// --- Mocks ---

type mockRunner struct {
	execFn func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error)
}

func (m *mockRunner) Exec(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
	return m.execFn(ctx, req)
}

type mockBuilder struct {
	firstRoundFn  func(input prompt.FirstRoundInput) (string, error)
	resumeFn      func(input prompt.ResumeInput) (string, error)
	formatFn      func(findings []session.Finding) string
}

func (m *mockBuilder) BuildFirstRound(input prompt.FirstRoundInput) (string, error) {
	if m.firstRoundFn != nil {
		return m.firstRoundFn(input)
	}
	return "first-round-prompt", nil
}

func (m *mockBuilder) BuildResume(input prompt.ResumeInput) (string, error) {
	if m.resumeFn != nil {
		return m.resumeFn(input)
	}
	return "resume-prompt", nil
}

func (m *mockBuilder) FormatFindingsForPrompt(findings []session.Finding) string {
	if m.formatFn != nil {
		return m.formatFn(findings)
	}
	return "formatted-findings"
}

type mockParser struct {
	parseFn func(stdout string) (*session.CodexResponse, error)
}

func (m *mockParser) Parse(stdout string) (*session.CodexResponse, error) {
	return m.parseFn(stdout)
}

type mockManager struct {
	sessions map[string]*session.Session
	createFn func(targets []string, targetMode, ctx string, cfg *config.Config) (*session.Session, error)
}

func newMockManager() *mockManager {
	return &mockManager{sessions: make(map[string]*session.Session)}
}

func (m *mockManager) Create(targets []string, targetMode, ctx string, cfg *config.Config) (*session.Session, error) {
	if m.createFn != nil {
		return m.createFn(targets, targetMode, ctx, cfg)
	}
	sess := &session.Session{
		SessionID:  "xr-test-001",
		CodexModel: cfg.CodexModel,
		Context:    ctx,
		Targets:    targets,
		TargetMode: targetMode,
		Status:     session.StatusInitialized,
	}
	m.sessions[sess.SessionID] = sess
	return sess, nil
}

func (m *mockManager) Load(id string) (*session.Session, error) {
	if s, ok := m.sessions[id]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("session %q not found", id)
}

func (m *mockManager) Update(sess *session.Session) error {
	m.sessions[sess.SessionID] = sess
	return nil
}

func (m *mockManager) Delete(id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockManager) List() ([]string, error) {
	var ids []string
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

type mockCollector struct {
	files []collector.FileContent
	err   error
}

func (m *mockCollector) Collect(ctx context.Context, targets []string, mode string) ([]collector.FileContent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.files, nil
}

// --- Tests ---

func TestReview_HappyPath(t *testing.T) {
	mgr := newMockManager()
	coll := &mockCollector{
		files: []collector.FileContent{
			{Path: "main.go", Content: "package main\n", Lines: 1},
		},
	}
	runner := &mockRunner{
		execFn: func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
			return &codex.ExecResult{
				Stdout:         `{"verdict":"NEEDS_REVIEW","summary":"found issues","findings":[{"id":"F001","severity":"high","category":"security","file":"main.go","line":1,"description":"test issue","suggestion":"fix it"}]}`,
				CodexSessionID: "codex-sess-123",
				DurationMs:     500,
			}, nil
		},
	}
	psr := &mockParser{
		parseFn: func(stdout string) (*session.CodexResponse, error) {
			return &session.CodexResponse{
				Verdict: "NEEDS_REVIEW",
				Summary: "found issues",
				Findings: []session.CodexFinding{
					{ID: "F001", Severity: "high", Category: "security", File: "main.go", Line: 1, Description: "test issue", Suggestion: "fix it"},
				},
			}, nil
		},
	}
	bldr := &mockBuilder{}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 600}

	r := NewSingleReviewer(runner, bldr, psr, mgr, coll, cfg, "/tmp/test-workdir")

	result, err := r.Review(context.Background(), ReviewRequest{
		Targets:    []string{"main.go"},
		TargetMode: "files",
		Context:    "test context",
		Timeout:    60,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SessionID != "xr-test-001" {
		t.Errorf("expected session ID xr-test-001, got %s", result.SessionID)
	}
	if result.Round != 1 {
		t.Errorf("expected round 1, got %d", result.Round)
	}
	if result.Verdict != "NEEDS_REVIEW" {
		t.Errorf("expected NEEDS_REVIEW, got %s", result.Verdict)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].ID != "F001" {
		t.Errorf("expected finding F001, got %s", result.Findings[0].ID)
	}
	if result.Summary.Total != 1 || result.Summary.Open != 1 {
		t.Errorf("unexpected summary: %+v", result.Summary)
	}

	// Verify session was updated
	sess := mgr.sessions["xr-test-001"]
	if sess.CodexSessionID != "codex-sess-123" {
		t.Errorf("expected codex session ID saved, got %s", sess.CodexSessionID)
	}
	if sess.Status != session.StatusInReview {
		t.Errorf("expected status in_review, got %s", sess.Status)
	}
}

func TestReview_CollectorError(t *testing.T) {
	mgr := newMockManager()
	coll := &mockCollector{err: fmt.Errorf("no files found")}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 600}

	r := NewSingleReviewer(&mockRunner{}, &mockBuilder{}, &mockParser{parseFn: nil}, mgr, coll, cfg, "/tmp/test-workdir")

	_, err := r.Review(context.Background(), ReviewRequest{
		Targets:    []string{"missing.go"},
		TargetMode: "files",
	})
	if err == nil {
		t.Fatal("expected error from collector")
	}
}

func TestReview_CodexError(t *testing.T) {
	mgr := newMockManager()
	coll := &mockCollector{
		files: []collector.FileContent{{Path: "main.go", Content: "pkg main\n", Lines: 1}},
	}
	runner := &mockRunner{
		execFn: func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
			return nil, fmt.Errorf("codex timed out")
		},
	}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 600}

	r := NewSingleReviewer(runner, &mockBuilder{}, &mockParser{parseFn: nil}, mgr, coll, cfg, "/tmp/test-workdir")

	_, err := r.Review(context.Background(), ReviewRequest{
		Targets:    []string{"main.go"},
		TargetMode: "files",
	})
	if err == nil {
		t.Fatal("expected error from codex")
	}
}

func TestVerify_HappyPath(t *testing.T) {
	mgr := newMockManager()
	// Pre-populate a session from a previous review
	mgr.sessions["xr-test-001"] = &session.Session{
		SessionID:      "xr-test-001",
		CodexSessionID: "codex-sess-123",
		CodexModel:     "gpt-5.3-Codex",
		Status:         session.StatusInReview,
		Round:          1,
		Targets:        []string{"main.go"},
		TargetMode:     "files",
		Findings: []session.Finding{
			{ID: "F001", Severity: "high", Category: "security", Status: session.FindingOpen, File: "main.go", Line: 1, Description: "SQL injection"},
		},
	}

	coll := &mockCollector{
		files: []collector.FileContent{{Path: "main.go", Content: "package main\n// fixed\n", Lines: 2}},
	}
	runner := &mockRunner{
		execFn: func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
			// Should resume the existing session
			if req.ResumeSessionID != "codex-sess-123" {
				t.Errorf("expected resume session ID codex-sess-123, got %s", req.ResumeSessionID)
			}
			return &codex.ExecResult{
				Stdout:         `{"verdict":"APPROVED","summary":"all fixed","findings":[{"id":"F001","status":"fixed","verification_note":"Fix looks correct"}]}`,
				CodexSessionID: "codex-sess-123",
			}, nil
		},
	}
	psr := &mockParser{
		parseFn: func(stdout string) (*session.CodexResponse, error) {
			return &session.CodexResponse{
				Verdict: "APPROVED",
				Summary: "all fixed",
				Findings: []session.CodexFinding{
					{ID: "F001", Status: "fixed", VerificationNote: "Fix looks correct"},
				},
			}, nil
		},
	}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 600}

	r := NewSingleReviewer(runner, &mockBuilder{}, psr, mgr, coll, cfg, "/tmp/test-workdir")

	result, err := r.Verify(context.Background(), VerifyRequest{
		SessionID: "xr-test-001",
		Message:   "Fixed the SQL injection by using parameterized queries",
		Timeout:   60,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Round != 2 {
		t.Errorf("expected round 2, got %d", result.Round)
	}
	if result.Verdict != "APPROVED" {
		t.Errorf("expected APPROVED, got %s", result.Verdict)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].Status != "fixed" {
		t.Errorf("expected finding status fixed, got %s", result.Findings[0].Status)
	}

	// Verify session state
	sess := mgr.sessions["xr-test-001"]
	if sess.Status != session.StatusVerifying {
		t.Errorf("expected status verifying, got %s", sess.Status)
	}
}

func TestVerify_FullRescan(t *testing.T) {
	mgr := newMockManager()
	mgr.sessions["xr-test-001"] = &session.Session{
		SessionID:      "xr-test-001",
		CodexSessionID: "codex-sess-123",
		CodexModel:     "gpt-5.3-Codex",
		Status:         session.StatusInReview,
		Round:          1,
		Targets:        []string{"main.go"},
		TargetMode:     "files",
		Findings:       []session.Finding{},
	}

	coll := &mockCollector{
		files: []collector.FileContent{{Path: "main.go", Content: "package main\n", Lines: 1}},
	}
	runner := &mockRunner{
		execFn: func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
			// Should NOT resume when fullRescan is true
			if req.ResumeSessionID != "" {
				t.Errorf("expected empty resume session ID for full rescan, got %s", req.ResumeSessionID)
			}
			return &codex.ExecResult{Stdout: `{"verdict":"APPROVED","summary":"clean","findings":[]}`}, nil
		},
	}
	psr := &mockParser{
		parseFn: func(stdout string) (*session.CodexResponse, error) {
			return &session.CodexResponse{Verdict: "APPROVED", Summary: "clean"}, nil
		},
	}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 600}

	r := NewSingleReviewer(runner, &mockBuilder{}, psr, mgr, coll, cfg, "/tmp/test-workdir")

	_, err := r.Verify(context.Background(), VerifyRequest{
		SessionID:  "xr-test-001",
		Message:    "Full rescan",
		FullRescan: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerify_SessionNotFound(t *testing.T) {
	mgr := newMockManager()
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 600}

	r := NewSingleReviewer(&mockRunner{}, &mockBuilder{}, &mockParser{}, mgr, &mockCollector{}, cfg, "/tmp/test-workdir")

	_, err := r.Verify(context.Background(), VerifyRequest{
		SessionID: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

// --- Helper tests ---

func TestMergeFindings_UpdateExisting(t *testing.T) {
	existing := []session.Finding{
		{ID: "F001", Status: "open", Description: "old desc"},
		{ID: "F002", Status: "open", Description: "unchanged"},
	}
	incoming := []session.CodexFinding{
		{ID: "F001", Status: "fixed", VerificationNote: "looks good"},
	}

	result := mergeFindings(existing, incoming)

	if len(result) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(result))
	}
	if result[0].Status != "fixed" {
		t.Errorf("expected F001 status fixed, got %s", result[0].Status)
	}
	if result[0].VerificationNote != "looks good" {
		t.Errorf("expected verification note, got %s", result[0].VerificationNote)
	}
	if result[1].Status != "open" {
		t.Errorf("F002 should be unchanged, got status %s", result[1].Status)
	}
}

func TestMergeFindings_AddNew(t *testing.T) {
	existing := []session.Finding{
		{ID: "F001", Status: "fixed"},
	}
	incoming := []session.CodexFinding{
		{ID: "F001", Status: "fixed"},
		{ID: "F003", Severity: "medium", Category: "logic", File: "new.go", Line: 10, Description: "new issue"},
	}

	result := mergeFindings(existing, incoming)

	if len(result) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(result))
	}
	if result[1].ID != "F003" {
		t.Errorf("expected new finding F003, got %s", result[1].ID)
	}
}

func TestCodexFindingsToFindings_DefaultStatus(t *testing.T) {
	cf := []session.CodexFinding{
		{ID: "F001", Severity: "high", File: "main.go", Line: 1, Description: "issue"},
	}

	result := codexFindingsToFindings(cf)

	if result[0].Status != session.FindingOpen {
		t.Errorf("expected default status 'open', got %s", result[0].Status)
	}
}

func TestCodexFindingsToFindings_EnrichedFields(t *testing.T) {
	cf := []session.CodexFinding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			File:        "db.go",
			Line:        19,
			Description: "SQL injection",
			Suggestion:  "Use parameterized query",
			Trigger:     "attacker sends malicious id",
			CascadeImpact: []string{
				"handler/task.go:GetTaskHandler() — passes input",
			},
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "Parameterized query", Effort: "minimal", Recommended: true},
			},
		},
	}

	result := codexFindingsToFindings(cf)

	if result[0].Trigger != "attacker sends malicious id" {
		t.Errorf("trigger mismatch: got %q", result[0].Trigger)
	}
	if len(result[0].CascadeImpact) != 1 {
		t.Fatalf("expected 1 cascade impact, got %d", len(result[0].CascadeImpact))
	}
	if len(result[0].FixAlternatives) != 1 {
		t.Fatalf("expected 1 fix alternative, got %d", len(result[0].FixAlternatives))
	}
	if result[0].FixAlternatives[0].Label != "A" {
		t.Errorf("alternative label mismatch: got %q", result[0].FixAlternatives[0].Label)
	}
}

func TestMergeFindings_UpdateEnrichedFields(t *testing.T) {
	existing := []session.Finding{
		{
			ID:       "F001",
			Status:   "open",
			Trigger:  "old trigger",
			CascadeImpact: []string{"old cascade"},
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "old fix", Effort: "minimal", Recommended: true},
			},
		},
	}
	incoming := []session.CodexFinding{
		{
			ID:       "F001",
			Status:   "open",
			Trigger:  "updated trigger",
			CascadeImpact: []string{"new cascade 1", "new cascade 2"},
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "updated fix", Effort: "minimal", Recommended: true},
				{Label: "B", Description: "new option", Effort: "large", Recommended: false},
			},
		},
	}

	result := mergeFindings(existing, incoming)

	f := result[0]
	if f.Trigger != "updated trigger" {
		t.Errorf("trigger not updated: got %q", f.Trigger)
	}
	if len(f.CascadeImpact) != 2 {
		t.Fatalf("expected 2 cascade impacts, got %d", len(f.CascadeImpact))
	}
	if len(f.FixAlternatives) != 2 {
		t.Fatalf("expected 2 alternatives, got %d", len(f.FixAlternatives))
	}
}

func TestMergeFindings_AddNewWithEnrichedFields(t *testing.T) {
	existing := []session.Finding{}
	incoming := []session.CodexFinding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			File:        "db.go",
			Line:        19,
			Description: "SQL injection",
			Trigger:     "malicious input",
			CascadeImpact: []string{"handler.go:Handle()"},
			FixAlternatives: []session.FixAlternative{
				{Label: "A", Description: "fix", Effort: "minimal", Recommended: true},
			},
		},
	}

	result := mergeFindings(existing, incoming)

	if len(result) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result))
	}
	if result[0].Trigger != "malicious input" {
		t.Errorf("trigger mismatch: got %q", result[0].Trigger)
	}
	if len(result[0].FixAlternatives) != 1 {
		t.Fatalf("expected 1 alternative, got %d", len(result[0].FixAlternatives))
	}
}

func TestReview_EnrichedFieldsEndToEnd(t *testing.T) {
	mgr := newMockManager()
	coll := &mockCollector{
		files: []collector.FileContent{
			{Path: "db.go", Content: "package db\nfunc GetTask(id string) {}\n", Lines: 2},
		},
	}
	runner := &mockRunner{
		execFn: func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
			return &codex.ExecResult{
				Stdout:         "{}",
				CodexSessionID: "codex-sess-456",
				DurationMs:     300,
			}, nil
		},
	}
	psr := &mockParser{
		parseFn: func(stdout string) (*session.CodexResponse, error) {
			return &session.CodexResponse{
				Verdict: "REVISE",
				Summary: "found security issue",
				Findings: []session.CodexFinding{
					{
						ID:          "F001",
						Severity:    "high",
						Category:    "security",
						File:        "db.go",
						Line:        2,
						Description: "SQL injection in GetTask",
						Suggestion:  "Use parameterized query",
						CodeSnippet: "func GetTask(id string) {}",
						Trigger:     "attacker sends id=' OR '1'='1",
						CascadeImpact: []string{
							"handler/task.go:GetTaskHandler() — passes user input directly",
						},
						FixAlternatives: []session.FixAlternative{
							{Label: "A", Description: "Parameterized query", Effort: "minimal", Recommended: true},
							{Label: "B", Description: "ORM layer", Effort: "large", Recommended: false},
						},
					},
				},
			}, nil
		},
	}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 600}

	r := NewSingleReviewer(runner, &mockBuilder{}, psr, mgr, coll, cfg, "/tmp/test-workdir")

	result, err := r.Review(context.Background(), ReviewRequest{
		Targets:    []string{"db.go"},
		TargetMode: "files",
		Context:    "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}

	f := result.Findings[0]
	if f.Trigger != "attacker sends id=' OR '1'='1" {
		t.Errorf("trigger not carried: got %q", f.Trigger)
	}
	if len(f.CascadeImpact) != 1 {
		t.Errorf("cascade not carried: got %d items", len(f.CascadeImpact))
	}
	if len(f.FixAlternatives) != 2 {
		t.Errorf("alternatives not carried: got %d items", len(f.FixAlternatives))
	}
	if !f.FixAlternatives[0].Recommended {
		t.Errorf("expected first alternative to be recommended")
	}

	// Verify session state also has enriched fields
	sess := mgr.sessions[result.SessionID]
	if sess.Findings[0].Trigger == "" {
		t.Error("session finding should have trigger")
	}
}

func TestBuildFetchMethod_Files(t *testing.T) {
	result := buildFetchMethod([]string{"main.go", "util.go"}, "files")

	if !strings.Contains(result, "main.go") {
		t.Error("expected main.go in fetch method")
	}
	if !strings.Contains(result, "util.go") {
		t.Error("expected util.go in fetch method")
	}
	if strings.Contains(result, "git diff") {
		t.Error("files mode should not use git diff")
	}
	if !strings.Contains(result, "Read the following files") {
		t.Error("expected read instruction")
	}
}

func TestBuildFetchMethod_GitUncommitted(t *testing.T) {
	result := buildFetchMethod(nil, "git-uncommitted")

	if !strings.Contains(result, "git diff") {
		t.Error("expected git diff in fetch method")
	}
	if !strings.Contains(result, "git diff --cached") {
		t.Error("expected git diff --cached in fetch method")
	}
	if !strings.Contains(result, "git ls-files") {
		t.Error("expected git ls-files in fetch method")
	}
}

func TestBuildFileListSummary(t *testing.T) {
	files := []collector.FileContent{
		{Path: "main.go", Content: "package main\n", Lines: 1},
		{Path: "util.go", Content: "package util\n", Lines: 15},
	}

	result := buildFileListSummary(files)

	if !strings.Contains(result, "main.go (1 lines)") {
		t.Errorf("expected main.go (1 lines), got: %s", result)
	}
	if !strings.Contains(result, "util.go (15 lines)") {
		t.Errorf("expected util.go (15 lines), got: %s", result)
	}
}

func TestReview_NoFileContentInPrompt(t *testing.T) {
	mgr := newMockManager()
	coll := &mockCollector{
		files: []collector.FileContent{
			{Path: "main.go", Content: "package main\n", Lines: 1},
		},
	}

	var capturedPromptInput prompt.FirstRoundInput
	bldr := &mockBuilder{
		firstRoundFn: func(input prompt.FirstRoundInput) (string, error) {
			capturedPromptInput = input
			return "prompt", nil
		},
	}

	runner := &mockRunner{
		execFn: func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
			return &codex.ExecResult{
				Stdout:         "{}",
				CodexSessionID: "codex-sess-789",
			}, nil
		},
	}
	psr := &mockParser{
		parseFn: func(stdout string) (*session.CodexResponse, error) {
			return &session.CodexResponse{Verdict: "APPROVED"}, nil
		},
	}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 600}

	r := NewSingleReviewer(runner, bldr, psr, mgr, coll, cfg, "/tmp/test-workdir")

	_, err := r.Review(context.Background(), ReviewRequest{
		Targets:    []string{"main.go"},
		TargetMode: "files",
		Context:    "test context",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FetchMethod should instruct Codex to read files (not git diff)
	if capturedPromptInput.FetchMethod == "" {
		t.Error("expected FetchMethod to be set")
	}
	if !strings.Contains(capturedPromptInput.FetchMethod, "main.go") {
		t.Errorf("expected FetchMethod to reference target file, got: %s", capturedPromptInput.FetchMethod)
	}
	if strings.Contains(capturedPromptInput.FetchMethod, "git diff") {
		t.Error("files mode should not use git diff")
	}
	if capturedPromptInput.FileList == "" {
		t.Error("expected FileList to be set")
	}
}

func TestReview_GitUncommitted_UseGitDiff(t *testing.T) {
	mgr := newMockManager()
	coll := &mockCollector{
		files: []collector.FileContent{
			{Path: "main.go", Content: "package main\n", Lines: 1},
		},
	}

	var capturedPromptInput prompt.FirstRoundInput
	bldr := &mockBuilder{
		firstRoundFn: func(input prompt.FirstRoundInput) (string, error) {
			capturedPromptInput = input
			return "prompt", nil
		},
	}

	runner := &mockRunner{
		execFn: func(ctx context.Context, req codex.ExecRequest) (*codex.ExecResult, error) {
			return &codex.ExecResult{Stdout: "{}", CodexSessionID: "codex-sess-789"}, nil
		},
	}
	psr := &mockParser{
		parseFn: func(stdout string) (*session.CodexResponse, error) {
			return &session.CodexResponse{Verdict: "APPROVED"}, nil
		},
	}
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 600}

	r := NewSingleReviewer(runner, bldr, psr, mgr, coll, cfg, "/tmp/test-workdir")

	_, err := r.Review(context.Background(), ReviewRequest{
		TargetMode: "git-uncommitted",
		Context:    "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// git-uncommitted mode should use git diff commands
	if !strings.Contains(capturedPromptInput.FetchMethod, "git diff") {
		t.Errorf("expected git diff in FetchMethod, got: %s", capturedPromptInput.FetchMethod)
	}
}
