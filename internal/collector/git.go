package collector

import "fmt"

// GitUncommittedFiles returns the paths of all uncommitted files (staged + unstaged)
// in the git repository at workdir.
func GitUncommittedFiles(workdir string) ([]string, error) {
	// TODO: implement — run git diff --name-only + git diff --cached --name-only
	return nil, fmt.Errorf("git uncommitted: not implemented")
}
