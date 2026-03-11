package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/davidleitw/xreview/internal/config"
)

// skipDirs are directories always skipped during recursive traversal.
var skipDirs = map[string]bool{
	".git":      true,
	".hg":       true,
	".svn":      true,
	".xreview":  true,
	"node_modules": true,
	"__pycache__":  true,
}

// FileContent holds a file's path and its content with line numbers.
type FileContent struct {
	Path    string
	Content string
	Lines   int
}

// Collector reads file contents for review.
type Collector interface {
	// Collect reads the content of the given targets.
	// mode is "files" or "git-uncommitted".
	// Directories in targets are expanded recursively, respecting ignore patterns.
	Collect(ctx context.Context, targets []string, mode string) ([]FileContent, error)
}

type collector struct {
	cfg     *config.Config
	workdir string
}

// NewCollector creates a Collector that respects the given config's ignore patterns.
func NewCollector(cfg *config.Config, workdir string) Collector {
	return &collector{cfg: cfg, workdir: workdir}
}

func (c *collector) Collect(ctx context.Context, targets []string, mode string) ([]FileContent, error) {
	var paths []string

	switch mode {
	case "git-uncommitted":
		files, err := GitUncommittedFiles(c.workdir)
		if err != nil {
			return nil, fmt.Errorf("collect git-uncommitted: %w", err)
		}
		paths = files
	case "files":
		expanded, err := c.expandTargets(targets)
		if err != nil {
			return nil, fmt.Errorf("collect files: %w", err)
		}
		paths = expanded
	default:
		return nil, fmt.Errorf("unknown collect mode: %q", mode)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no files to review")
	}

	var result []FileContent
	for _, p := range paths {
		if c.isIgnored(p) {
			continue
		}

		content, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}

		// Skip binary / non-UTF-8 files
		if !utf8.Valid(content) {
			continue
		}

		lines := strings.Count(string(content), "\n")
		if len(content) > 0 && content[len(content)-1] != '\n' {
			lines++
		}

		result = append(result, FileContent{
			Path:    p,
			Content: string(content),
			Lines:   lines,
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no files to review after filtering")
	}

	return result, nil
}

// expandTargets resolves file paths and expands directories recursively.
func (c *collector) expandTargets(targets []string) ([]string, error) {
	var paths []string

	for _, t := range targets {
		abs := t
		if !filepath.IsAbs(t) {
			abs = filepath.Join(c.workdir, t)
		}

		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("file not found: %s", t)
		}

		if !info.IsDir() {
			paths = append(paths, abs)
			continue
		}

		err = filepath.Walk(abs, func(path string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if fi.IsDir() {
				if skipDirs[fi.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			paths = append(paths, path)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", t, err)
		}
	}

	return paths, nil
}

// isIgnored checks if a path matches any configured ignore pattern.
func (c *collector) isIgnored(path string) bool {
	for _, pattern := range c.cfg.IgnorePatterns {
		rel, err := filepath.Rel(c.workdir, path)
		if err != nil {
			rel = path
		}
		matched, _ := filepath.Match(pattern, rel)
		if matched {
			return true
		}
		// Also try matching just the filename for simple patterns.
		matched, _ = filepath.Match(pattern, filepath.Base(path))
		if matched {
			return true
		}
	}
	return false
}
