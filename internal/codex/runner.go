package codex

import (
	"context"
	"fmt"
	"time"
)

// ExecRequest holds parameters for a codex exec call.
type ExecRequest struct {
	Model           string
	Prompt          string
	SchemaPath      string
	Timeout         time.Duration
	ResumeSessionID string // empty for new sessions
}

// ExecResult holds the output from a codex exec call.
type ExecResult struct {
	Stdout         string
	Stderr         string
	CodexSessionID string
	DurationMs     int64
}

// Runner executes codex as a subprocess.
type Runner interface {
	Exec(ctx context.Context, req ExecRequest) (*ExecResult, error)
}

type runner struct{}

// NewRunner creates a Runner.
func NewRunner() Runner {
	return &runner{}
}

func (r *runner) Exec(ctx context.Context, req ExecRequest) (*ExecResult, error) {
	// TODO: implement — spawn codex exec subprocess, capture stdout/stderr
	return nil, fmt.Errorf("codex runner: not implemented")
}
