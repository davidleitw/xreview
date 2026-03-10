package reviewer

import (
	"context"
	"fmt"

	"github.com/davidleitw/xreview/internal/codex"
	"github.com/davidleitw/xreview/internal/collector"
	"github.com/davidleitw/xreview/internal/parser"
	"github.com/davidleitw/xreview/internal/prompt"
	"github.com/davidleitw/xreview/internal/session"
)

// SingleReviewer uses a single codex call for review.
type SingleReviewer struct {
	runner    codex.Runner
	builder   prompt.Builder
	parser    parser.Parser
	sessions  session.Manager
	collector collector.Collector
}

// NewSingleReviewer creates a SingleReviewer with the given dependencies.
func NewSingleReviewer(
	runner codex.Runner,
	builder prompt.Builder,
	parser parser.Parser,
	sessions session.Manager,
	collector collector.Collector,
) *SingleReviewer {
	return &SingleReviewer{
		runner:    runner,
		builder:   builder,
		parser:    parser,
		sessions:  sessions,
		collector: collector,
	}
}

func (r *SingleReviewer) Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	// TODO: implement
	return nil, fmt.Errorf("review: not implemented")
}

func (r *SingleReviewer) Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	// TODO: implement
	return nil, fmt.Errorf("verify: not implemented")
}
