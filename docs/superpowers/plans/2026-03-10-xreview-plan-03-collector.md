# Plan 3: Collector — File Reading + Git Uncommitted

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collect file contents with line numbers for review prompts, and detect uncommitted changes via git.

**Architecture:** `internal/collector/` package. Two modes: explicit file paths, or git uncommitted detection. Output is a structured list of file contents with line numbers + unified diff.

**Tech Stack:** Go stdlib (`os`, `bufio`, `os/exec`, `strings`, `fmt`)

**Depends on:** Plan 1

---

## Chunk 1: File Collector + Git Integration

### File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `internal/collector/collector.go` | Read files, format with line numbers, generate file list summary |
| Create | `internal/collector/collector_test.go` | File reading tests |
| Create | `internal/collector/git.go` | Git uncommitted file detection + unified diff |
| Create | `internal/collector/git_test.go` | Git integration tests |

---

### Task 3.1: File Content Collector

**Files:**
- Create: `internal/collector/collector.go`
- Test: `internal/collector/collector_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/collector/collector_test.go`:

```go
package collector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectFiles(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "auth.go"), []byte("package main\n\nfunc auth() bool {\n\treturn true\n}\n"), 0644)

	files := []string{
		filepath.Join(dir, "main.go"),
		filepath.Join(dir, "auth.go"),
	}

	result, err := CollectFiles(files)
	if err != nil {
		t.Fatalf("CollectFiles: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}

	// Check line numbers are present
	if !strings.Contains(result[0].ContentWithLineNumbers, "     1\t") {
		t.Error("expected line numbers in content")
	}
	if result[0].LineCount != 5 {
		t.Errorf("line count: got %d, want 5", result[0].LineCount)
	}
}

func TestCollectFiles_FileNotFound(t *testing.T) {
	_, err := CollectFiles([]string{"/nonexistent/file.go"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestFormatLineNumbers(t *testing.T) {
	input := "line one\nline two\nline three\n"
	expected := "     1\tline one\n     2\tline two\n     3\tline three\n"

	got := FormatWithLineNumbers(input)
	if got != expected {
		t.Errorf("got:\n%s\nwant:\n%s", got, expected)
	}
}

func TestFormatFileList(t *testing.T) {
	files := []CollectedFile{
		{Path: "src/auth.go", LineCount: 50},
		{Path: "src/middleware.go", LineCount: 30},
	}

	result := FormatFileList(files)

	if !strings.Contains(result, "src/auth.go") {
		t.Error("missing auth.go in file list")
	}
	if !strings.Contains(result, "src/middleware.go") {
		t.Error("missing middleware.go in file list")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/collector/ -v`
Expected: FAIL — package not defined

- [ ] **Step 3: Write collector.go**

Create `internal/collector/collector.go`:

```go
package collector

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// CollectedFile holds a file's path and content for review.
type CollectedFile struct {
	Path                   string
	Content                string
	ContentWithLineNumbers string
	LineCount              int
}

// CollectFiles reads the given file paths and returns their contents with line numbers.
func CollectFiles(paths []string) ([]CollectedFile, error) {
	var files []CollectedFile
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("file not found: %s", p)
		}
		content := string(data)
		lineCount := countLines(content)
		files = append(files, CollectedFile{
			Path:                   p,
			Content:                content,
			ContentWithLineNumbers: FormatWithLineNumbers(content),
			LineCount:              lineCount,
		})
	}
	return files, nil
}

// FormatWithLineNumbers adds line numbers to content (cat -n style).
func FormatWithLineNumbers(content string) string {
	var b strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		fmt.Fprintf(&b, "%6d\t%s\n", lineNum, scanner.Text())
	}
	return b.String()
}

// FormatFileList generates a summary of files being reviewed.
func FormatFileList(files []CollectedFile) string {
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "- %s (%d lines)\n", f.Path, f.LineCount)
	}
	return b.String()
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/collector/ -v`
Expected: All 4 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/collector/collector.go internal/collector/collector_test.go
git commit -m "feat: add file collector with line number formatting"
```

---

### Task 3.2: Git Uncommitted Detection

**Files:**
- Create: `internal/collector/git.go`
- Test: `internal/collector/git_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/collector/git_test.go`:

```go
package collector

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Init git repo
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}

	// Create and commit a base file
	os.WriteFile(filepath.Join(dir, "base.go"), []byte("package main\n"), 0644)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = dir
	cmd.Run()

	return dir
}

func TestGitUncommittedFiles(t *testing.T) {
	dir := setupGitRepo(t)

	// Create uncommitted changes
	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\nfunc New() {}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "base.go"), []byte("package main\n// modified\n"), 0644)

	// Stage new.go
	cmd := exec.Command("git", "add", "new.go")
	cmd.Dir = dir
	cmd.Run()

	files, err := GitUncommittedFiles(dir)
	if err != nil {
		t.Fatalf("GitUncommittedFiles: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("expected at least 2 files, got %d: %v", len(files), files)
	}

	// Both should be present
	found := map[string]bool{}
	for _, f := range files {
		found[filepath.Base(f)] = true
	}
	if !found["new.go"] {
		t.Error("missing new.go")
	}
	if !found["base.go"] {
		t.Error("missing base.go")
	}
}

func TestGitUncommittedFiles_NoChanges(t *testing.T) {
	dir := setupGitRepo(t)

	files, err := GitUncommittedFiles(dir)
	if err != nil {
		t.Fatalf("GitUncommittedFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d: %v", len(files), files)
	}
}

func TestGitUncommittedFiles_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := GitUncommittedFiles(dir)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
}

func TestGitDiff(t *testing.T) {
	dir := setupGitRepo(t)

	// Modify a file
	os.WriteFile(filepath.Join(dir, "base.go"), []byte("package main\n// modified\n"), 0644)

	diff, err := GitDiff(dir)
	if err != nil {
		t.Fatalf("GitDiff: %v", err)
	}

	if diff == "" {
		t.Error("expected non-empty diff")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/collector/ -v -run TestGit`
Expected: FAIL — functions not defined

- [ ] **Step 3: Write git.go**

Create `internal/collector/git.go`:

```go
package collector

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitUncommittedFiles returns absolute paths of all uncommitted files (staged + unstaged tracked).
func GitUncommittedFiles(workdir string) ([]string, error) {
	if !isGitRepo(workdir) {
		return nil, fmt.Errorf("not inside a git repository")
	}

	// Get staged files
	staged, err := gitOutput(workdir, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, fmt.Errorf("git diff --cached: %w", err)
	}

	// Get unstaged modified tracked files
	unstaged, err := gitOutput(workdir, "diff", "--name-only")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	// Get untracked files
	untracked, err := gitOutput(workdir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}

	seen := make(map[string]bool)
	var files []string

	for _, list := range []string{staged, unstaged, untracked} {
		for _, line := range strings.Split(strings.TrimSpace(list), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			abs := filepath.Join(workdir, line)
			if !seen[abs] {
				seen[abs] = true
				files = append(files, abs)
			}
		}
	}

	return files, nil
}

// GitDiff returns the unified diff of all uncommitted changes.
func GitDiff(workdir string) (string, error) {
	if !isGitRepo(workdir) {
		return "", fmt.Errorf("not inside a git repository")
	}

	// Staged diff
	staged, err := gitOutput(workdir, "diff", "--cached")
	if err != nil {
		return "", err
	}

	// Unstaged diff
	unstaged, err := gitOutput(workdir, "diff")
	if err != nil {
		return "", err
	}

	var parts []string
	if staged != "" {
		parts = append(parts, staged)
	}
	if unstaged != "" {
		parts = append(parts, unstaged)
	}
	return strings.Join(parts, "\n"), nil
}

func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/collector/ -v -run TestGit`
Expected: All 4 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/collector/git.go internal/collector/git_test.go
git commit -m "feat: add git uncommitted file detection and diff"
```
