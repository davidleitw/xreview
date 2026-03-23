package reviewer

import (
	"context"

	"github.com/davidleitw/xreview/internal/session"
)

// ReviewRequest holds parameters for starting a new review.
type ReviewRequest struct {
	Targets    []string
	TargetMode string // "files" or "git-uncommitted"
	Context    string
	Timeout    int
	Language   string // language key, e.g. "cpp". Empty = no language-specific guidelines.
}

// ReviewResult holds the output of a review round.
type ReviewResult struct {
	SessionID string
	Round     int
	Verdict   string
	Findings  []session.Finding
	Summary   session.FindingSummary
	Language  string
}

// VerifyRequest holds parameters for a follow-up verification round.
type VerifyRequest struct {
	SessionID       string
	Message         string
	FullRescan      bool
	Timeout         int
	ExtraTargets    []string // additional files to include in resume
	ExtraTargetMode string   // "files" or "git-uncommitted" for extra targets
}

// VerifyResult holds the output of a verification round.
type VerifyResult struct {
	SessionID string
	Round     int
	Verdict   string
	Findings  []session.Finding
	Summary   session.FindingSummary
	Language  string
}

// Reviewer abstracts single vs. multi-agent review.
// Day 1: SingleReviewer (one codex call).
// Future: MultiReviewer (parallel codex calls with aggregation).
type Reviewer interface {
	Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error)
	Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error)
}
