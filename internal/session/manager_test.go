package session

import (
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
