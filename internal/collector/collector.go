package collector

import (
	"context"
	"fmt"

	"github.com/davidleitw/xreview/internal/config"
)

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
	cfg *config.Config
}

// NewCollector creates a Collector that respects the given config's ignore patterns.
func NewCollector(cfg *config.Config) Collector {
	return &collector{cfg: cfg}
}

func (c *collector) Collect(ctx context.Context, targets []string, mode string) ([]FileContent, error) {
	// TODO: implement — expand directories, read files, add line numbers
	return nil, fmt.Errorf("collector: not implemented")
}
