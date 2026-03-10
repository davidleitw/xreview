package codex

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/davidleitw/xreview/internal/parser"
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

// BuildArgs constructs the codex exec command arguments from an ExecRequest.
func BuildArgs(req ExecRequest) []string {
	args := []string{"exec"}

	if req.Model != "" {
		args = append(args, "-m", req.Model)
	}

	if req.SchemaPath != "" {
		args = append(args, "--output-schema", req.SchemaPath)
	}

	args = append(args, "--skip-git-repo-check")
	args = append(args, "-c", "skills.allow_implicit_invocation=false")

	if req.ResumeSessionID != "" {
		args = append(args, "-r", req.ResumeSessionID)
	}

	args = append(args, req.Prompt)

	return args
}

func (r *runner) Exec(ctx context.Context, req ExecRequest) (*ExecResult, error) {
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	args := BuildArgs(req)
	cmd := exec.CommandContext(ctx, "codex", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	result := &ExecResult{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		DurationMs: duration,
	}

	// Extract codex session ID from stderr regardless of error
	if sessionID, extractErr := parser.ExtractCodexSessionID(stderr.String()); extractErr == nil {
		result.CodexSessionID = sessionID
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("codex exec timed out after %s", req.Timeout)
		}
		return result, fmt.Errorf("codex exec failed: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
	}

	return result, nil
}
