package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestSplitTargets(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"main.go", 1},
		{"main.go,util.go", 2},
		{"main.go, util.go , handler.go", 3},
		{"", 0},
		{" , , ", 0},
	}

	for _, tt := range tests {
		got := splitTargets(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitTargets(%q) = %d items, want %d", tt.input, len(got), tt.want)
		}
		for _, g := range got {
			if strings.TrimSpace(g) != g {
				t.Errorf("splitTargets(%q) has untrimmed item %q", tt.input, g)
			}
		}
	}
}

func TestReviewCmd_FlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no flags",
			args:    []string{},
			wantErr: "new review requires --files or --git-uncommitted",
		},
		{
			name:    "files and git-uncommitted",
			args:    []string{"--files", "a.go", "--git-uncommitted"},
			wantErr: "--files and --git-uncommitted are mutually exclusive",
		},
		{
			name:    "message without session",
			args:    []string{"--files", "a.go", "--message", "hi"},
			wantErr: "--message requires --session",
		},
		{
			name:    "full-rescan without session",
			args:    []string{"--files", "a.go", "--full-rescan"},
			wantErr: "--full-rescan requires --session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newRootCmd()
			root.SetArgs(append([]string{"review"}, tt.args...))
			err := root.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestReportCmd_RequiresSession(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"report"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--session is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCleanCmd_RequiresSession(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"clean"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--session is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReviewCmd_SessionWithFilesAllowed(t *testing.T) {
	root := newRootCmd()
	// This should pass PreRunE validation (will fail later in RunE because
	// session doesn't exist, but that's expected — we're testing flag validation)
	root.SetArgs([]string{"review", "--session", "xr-fake", "--files", "a.go", "--message", "check these too"})
	err := root.Execute()
	if err == nil {
		t.Skip("no real session, but PreRunE should have passed")
	}
	// Should NOT be the old mutual-exclusion error
	if strings.Contains(err.Error(), "--files/--git-uncommitted cannot be used with --session") {
		t.Error("--session + --files should now be allowed")
	}
}

func TestClassifyReviewError_DoesNotFalsePositiveOnCodexStderr(t *testing.T) {
	// Simulates codex failing with stderr that contains "session id: ..."
	// and an error message containing "not found" from a different context.
	// This is the real pattern: runner.go wraps stderr into the error message.
	err := fmt.Errorf("codex exited with error: exit status 1\nstderr: session id: 019cdb8c-6b73-79e3-8860-190f58f25ddc\ncommand not found: some-tool")
	code := classifyReviewError(err)
	if code == "SESSION_NOT_FOUND" {
		t.Errorf("should not classify codex stderr as SESSION_NOT_FOUND, got %s", code)
	}
}

func TestClassifyReviewError_RealSessionNotFound(t *testing.T) {
	// Simulates actual session-not-found from session/manager.go
	err := fmt.Errorf("load session: session %q not found", "xr-20260311-abc123")
	code := classifyReviewError(err)
	if code != "SESSION_NOT_FOUND" {
		t.Errorf("expected SESSION_NOT_FOUND, got %s", code)
	}
}

func TestVersionCmd(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"version"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("version should not error: %v", err)
	}
}
