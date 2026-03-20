package session

import (
	"strings"
	"testing"

	"github.com/davidleitw/xreview/internal/config"
)

func TestManager_CreateAndLoad(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex", DefaultTimeout: 180}

	sess, err := mgr.Create([]string{"main.go"}, "files", "test context", cfg)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if sess.SessionID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if sess.Status != StatusInitialized {
		t.Errorf("expected status initialized, got %s", sess.Status)
	}
	if sess.CodexModel != "gpt-5.3-Codex" {
		t.Errorf("expected model gpt-5.3-Codex, got %s", sess.CodexModel)
	}

	// Load it back
	loaded, err := mgr.Load(sess.SessionID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.SessionID != sess.SessionID {
		t.Errorf("loaded session ID mismatch: %s vs %s", loaded.SessionID, sess.SessionID)
	}
	if loaded.Context != "test context" {
		t.Errorf("expected context 'test context', got %s", loaded.Context)
	}
}

func TestManager_Update(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	sess, _ := mgr.Create([]string{"a.go"}, "files", "", cfg)
	sess.Status = StatusInReview
	sess.Round = 1
	sess.Findings = []Finding{
		{ID: "F001", Severity: "high", Status: FindingOpen, File: "a.go", Line: 1, Description: "issue"},
	}

	if err := mgr.Update(sess); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	loaded, _ := mgr.Load(sess.SessionID)
	if loaded.Status != StatusInReview {
		t.Errorf("expected in_review, got %s", loaded.Status)
	}
	if loaded.Round != 1 {
		t.Errorf("expected round 1, got %d", loaded.Round)
	}
	if len(loaded.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(loaded.Findings))
	}
}

func TestManager_Delete(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	sess, _ := mgr.Create([]string{"a.go"}, "files", "", cfg)

	if err := mgr.Delete(sess.SessionID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := mgr.Load(sess.SessionID)
	if err == nil {
		t.Fatal("expected error loading deleted session")
	}
}

func TestManager_List(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	mgr.Create([]string{"a.go"}, "files", "", cfg)
	mgr.Create([]string{"b.go"}, "files", "", cfg)

	ids, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(ids))
	}
}

func TestManager_LoadNotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	_, err := mgr.Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestManager_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	ids, err := mgr.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(ids))
	}
}

func TestManager_RoundTrip_EnrichedFindings(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	sess, err := mgr.Create([]string{"main.go"}, "files", "ctx", cfg)
	if err != nil {
		t.Fatal(err)
	}

	sess.Status = StatusInReview
	sess.Findings = []Finding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			Status:      FindingOpen,
			File:        "db.go",
			Line:        19,
			Description: "SQL injection",
			Suggestion:  "Use parameterized query",
			Trigger:     "attacker sends id=' OR '1'='1",
			CascadeImpact: []string{
				"handler/task.go:GetTaskHandler() — passes user input directly",
				"cache/task.go:GetCached() — bypasses DB validation",
			},
			FixAlternatives: []FixAlternative{
				{Label: "A", Description: "Parameterized query", Effort: "minimal", Recommended: true},
				{Label: "B", Description: "Introduce ORM", Effort: "large", Recommended: false},
			},
		},
	}

	if err := mgr.Update(sess); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	loaded, err := mgr.Load(sess.SessionID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	f := loaded.Findings[0]
	if f.Trigger != "attacker sends id=' OR '1'='1" {
		t.Errorf("trigger mismatch: got %q", f.Trigger)
	}
	if len(f.CascadeImpact) != 2 {
		t.Fatalf("expected 2 cascade impacts, got %d", len(f.CascadeImpact))
	}
	if f.CascadeImpact[0] != "handler/task.go:GetTaskHandler() — passes user input directly" {
		t.Errorf("cascade[0] mismatch: got %q", f.CascadeImpact[0])
	}
	if len(f.FixAlternatives) != 2 {
		t.Fatalf("expected 2 alternatives, got %d", len(f.FixAlternatives))
	}
	if f.FixAlternatives[0].Label != "A" || !f.FixAlternatives[0].Recommended {
		t.Errorf("alternative[0] mismatch: %+v", f.FixAlternatives[0])
	}
	if f.FixAlternatives[1].Effort != "large" {
		t.Errorf("alternative[1] effort mismatch: got %q", f.FixAlternatives[1].Effort)
	}
}

func TestManager_Load_RejectsOldVersion(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	sess, err := mgr.Create([]string{"a.go"}, "files", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	sess.Version = 0
	if err := mgr.Update(sess); err != nil {
		t.Fatal(err)
	}

	_, err = mgr.Load(sess.SessionID)
	if err == nil {
		t.Fatal("expected error loading session with old version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("expected error to contain 'version', got: %v", err)
	}
}

func TestManager_Load_RejectsFutureVersion(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	sess, err := mgr.Create([]string{"a.go"}, "files", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	sess.Version = 999
	if err := mgr.Update(sess); err != nil {
		t.Fatal(err)
	}

	_, err = mgr.Load(sess.SessionID)
	if err == nil {
		t.Fatal("expected error loading session with future version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("expected error to contain 'version', got: %v", err)
	}
}

func TestManager_RoundTrip_ConfidenceAndFixStrategy(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	cfg := &config.Config{CodexModel: "gpt-5.3-Codex"}

	sess, err := mgr.Create([]string{"main.go"}, "files", "ctx", cfg)
	if err != nil {
		t.Fatal(err)
	}

	sess.Findings = []Finding{
		{
			ID: "F001", Severity: "high", Category: "security",
			Status: FindingOpen, File: "a.go", Line: 10,
			Description: "issue1", Suggestion: "fix1",
			Confidence: 85, FixStrategy: "auto",
		},
		{
			ID: "F002", Severity: "low", Category: "style",
			Status: FindingOpen, File: "b.go", Line: 20,
			Description: "issue2", Suggestion: "fix2",
			Confidence: 40, FixStrategy: "ask",
		},
	}

	if err := mgr.Update(sess); err != nil {
		t.Fatal(err)
	}

	loaded, err := mgr.Load(sess.SessionID)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Findings[0].Confidence != 85 {
		t.Errorf("expected confidence 85, got %d", loaded.Findings[0].Confidence)
	}
	if loaded.Findings[0].FixStrategy != "auto" {
		t.Errorf("expected fix_strategy 'auto', got %q", loaded.Findings[0].FixStrategy)
	}
	if loaded.Findings[1].Confidence != 40 {
		t.Errorf("expected confidence 40, got %d", loaded.Findings[1].Confidence)
	}
	if loaded.Findings[1].FixStrategy != "ask" {
		t.Errorf("expected fix_strategy 'ask', got %q", loaded.Findings[1].FixStrategy)
	}
}

func TestSession_Summarize(t *testing.T) {
	sess := &Session{
		Findings: []Finding{
			{Status: FindingOpen},
			{Status: FindingFixed},
			{Status: FindingFixed},
			{Status: FindingDismissed},
			{Status: FindingReopened},
		},
	}

	sum := sess.Summarize()
	if sum.Total != 5 {
		t.Errorf("expected total 5, got %d", sum.Total)
	}
	if sum.Open != 2 { // open + reopened
		t.Errorf("expected open 2, got %d", sum.Open)
	}
	if sum.Fixed != 2 {
		t.Errorf("expected fixed 2, got %d", sum.Fixed)
	}
	if sum.Dismissed != 1 {
		t.Errorf("expected dismissed 1, got %d", sum.Dismissed)
	}
}
