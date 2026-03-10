package collector

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitUncommittedFiles_Unstaged(t *testing.T) {
	dir := setupGitRepo(t)

	// Create and commit a file
	writeFile(t, dir, "tracked.go", "package main\n")
	gitExec(t, dir, "add", "tracked.go")
	gitExec(t, dir, "commit", "-m", "init")

	// Modify it (unstaged change)
	writeFile(t, dir, "tracked.go", "package main\n// changed\n")

	files, err := GitUncommittedFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "tracked.go" {
		t.Errorf("expected tracked.go, got %s", files[0])
	}
}

func TestGitUncommittedFiles_Staged(t *testing.T) {
	dir := setupGitRepo(t)

	// Create, commit, modify, and stage
	writeFile(t, dir, "staged.go", "package main\n")
	gitExec(t, dir, "add", "staged.go")
	gitExec(t, dir, "commit", "-m", "init")
	writeFile(t, dir, "staged.go", "package main\n// staged\n")
	gitExec(t, dir, "add", "staged.go")

	files, err := GitUncommittedFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
}

func TestGitUncommittedFiles_Untracked(t *testing.T) {
	dir := setupGitRepo(t)

	// Initial commit so HEAD exists
	writeFile(t, dir, "init.go", "package main\n")
	gitExec(t, dir, "add", "init.go")
	gitExec(t, dir, "commit", "-m", "init")

	// Add untracked file
	writeFile(t, dir, "new.go", "package main\n")

	files, err := GitUncommittedFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "new.go" {
		t.Errorf("expected new.go, got %s", files[0])
	}
}

func TestGitUncommittedFiles_Dedup(t *testing.T) {
	dir := setupGitRepo(t)

	// Commit, then modify and stage (both staged + unstaged diff for same file)
	writeFile(t, dir, "dup.go", "package main\n")
	gitExec(t, dir, "add", "dup.go")
	gitExec(t, dir, "commit", "-m", "init")

	writeFile(t, dir, "dup.go", "package main\n// v2\n")
	gitExec(t, dir, "add", "dup.go")
	writeFile(t, dir, "dup.go", "package main\n// v3\n")

	files, err := GitUncommittedFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be deduplicated to 1
	if len(files) != 1 {
		t.Fatalf("expected 1 deduplicated file, got %d: %v", len(files), files)
	}
}

func TestGitUncommittedFiles_Clean(t *testing.T) {
	dir := setupGitRepo(t)

	writeFile(t, dir, "clean.go", "package main\n")
	gitExec(t, dir, "add", "clean.go")
	gitExec(t, dir, "commit", "-m", "init")

	files, err := GitUncommittedFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 0 {
		t.Fatalf("expected 0 files for clean repo, got %d: %v", len(files), files)
	}
}

func setupGitRepo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := t.TempDir()
	gitExec(t, dir, "init")
	gitExec(t, dir, "config", "user.email", "test@test.com")
	gitExec(t, dir, "config", "user.name", "Test")

	return dir
}

func gitExec(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2020-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2020-01-01T00:00:00Z")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
