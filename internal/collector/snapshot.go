package collector

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/davidleitw/xreview/internal/session"
)

// Snapshot computes SHA-256 checksums for all target files.
// Returned paths are relative to workdir.
// ignorePatterns applies the same filtering as Collect to keep snapshot scope
// consistent with review scope.
func Snapshot(targets []string, mode string, workdir string, ignorePatterns []string) ([]session.FileSnapshot, error) {
	var absPaths []string

	switch mode {
	case "git-uncommitted":
		files, err := GitUncommittedFiles(workdir)
		if err != nil {
			return nil, fmt.Errorf("snapshot git-uncommitted: %w", err)
		}
		absPaths = files
	case "files":
		for _, t := range targets {
			abs := t
			if !filepath.IsAbs(t) {
				abs = filepath.Join(workdir, t)
			}
			info, err := os.Stat(abs)
			if err != nil {
				continue // file may have been deleted
			}
			if info.IsDir() {
				_ = filepath.Walk(abs, func(path string, fi os.FileInfo, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if fi.IsDir() {
						if skipDirs[fi.Name()] {
							return filepath.SkipDir
						}
						return nil
					}
					absPaths = append(absPaths, path)
					return nil
				})
			} else {
				absPaths = append(absPaths, abs)
			}
		}
	default:
		return nil, fmt.Errorf("unknown snapshot mode: %q", mode)
	}

	var snapshots []session.FileSnapshot
	for _, abs := range absPaths {
		if isIgnoredPath(abs, workdir, ignorePatterns) {
			continue
		}
		checksum, err := checksumFile(abs)
		if err != nil {
			continue // file may be unreadable or deleted between list and read
		}
		rel, err := filepath.Rel(workdir, abs)
		if err != nil {
			rel = abs
		}
		snapshots = append(snapshots, session.FileSnapshot{
			Path:     rel,
			Checksum: checksum,
		})
	}

	return snapshots, nil
}

// isIgnoredPath checks if a path matches any of the given ignore patterns.
// Uses the same matching logic as collector.isIgnored.
func isIgnoredPath(path string, workdir string, patterns []string) bool {
	for _, pattern := range patterns {
		rel, err := filepath.Rel(workdir, path)
		if err != nil {
			rel = path
		}
		matched, _ := filepath.Match(pattern, rel)
		if matched {
			return true
		}
		matched, _ = filepath.Match(pattern, filepath.Base(path))
		if matched {
			return true
		}
	}
	return false
}

// FileChange describes how a file changed between two snapshots.
type FileChange struct {
	Path   string // relative to workdir
	Status string // "modified", "added", "deleted"
}

// DiffSnapshots compares old and new snapshots, returning the list of changes.
func DiffSnapshots(old, new []session.FileSnapshot) []FileChange {
	oldMap := make(map[string]string, len(old))
	for _, s := range old {
		oldMap[s.Path] = s.Checksum
	}

	newMap := make(map[string]string, len(new))
	for _, s := range new {
		newMap[s.Path] = s.Checksum
	}

	var changes []FileChange

	// Check for modified and added files
	for path, newHash := range newMap {
		if oldHash, ok := oldMap[path]; ok {
			if oldHash != newHash {
				changes = append(changes, FileChange{Path: path, Status: "modified"})
			}
		} else {
			changes = append(changes, FileChange{Path: path, Status: "added"})
		}
	}

	// Check for deleted files
	for path := range oldMap {
		if _, ok := newMap[path]; !ok {
			changes = append(changes, FileChange{Path: path, Status: "deleted"})
		}
	}

	return changes
}

func checksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
