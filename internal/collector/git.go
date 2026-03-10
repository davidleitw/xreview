package collector

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitUncommittedFiles returns the absolute paths of all uncommitted files
// (staged + unstaged) in the git repository at workdir.
func GitUncommittedFiles(workdir string) ([]string, error) {
	// Unstaged changes
	unstaged, err := gitDiffNames(workdir, false)
	if err != nil {
		return nil, err
	}

	// Staged changes
	staged, err := gitDiffNames(workdir, true)
	if err != nil {
		return nil, err
	}

	// Untracked files
	untracked, err := gitUntrackedFiles(workdir)
	if err != nil {
		return nil, err
	}

	// Merge and deduplicate
	seen := make(map[string]bool)
	var result []string
	for _, files := range [][]string{unstaged, staged, untracked} {
		for _, f := range files {
			abs := filepath.Join(workdir, f)
			if !seen[abs] {
				seen[abs] = true
				result = append(result, abs)
			}
		}
	}

	return result, nil
}

func gitDiffNames(workdir string, cached bool) ([]string, error) {
	args := []string{"diff", "--name-only"}
	if cached {
		args = append(args, "--cached")
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only: %w", err)
	}

	return splitLines(string(out)), nil
}

func gitUntrackedFiles(workdir string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = workdir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}

	return splitLines(string(out)), nil
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
