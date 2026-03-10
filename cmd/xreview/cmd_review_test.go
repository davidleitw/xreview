package main

import (
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
			name:    "files with session",
			args:    []string{"--files", "a.go", "--session", "s1"},
			wantErr: "--files/--git-uncommitted cannot be used with --session",
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

func TestVersionCmd(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"version"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("version should not error: %v", err)
	}
}
