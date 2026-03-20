package session

import "time"

// Session status constants.
const (
	StatusInitialized = "initialized"
	StatusInReview    = "in_review"
	StatusVerifying   = "verifying"
	StatusCompleted   = "completed"
)

// Finding status constants.
const (
	FindingOpen      = "open"
	FindingFixed     = "fixed"
	FindingDismissed = "dismissed"
	FindingReopened  = "reopened"
)

const CurrentSessionVersion = 2

// FileSnapshot records a file's checksum at a given round.
type FileSnapshot struct {
	Path     string `json:"path"`     // relative to workdir
	Checksum string `json:"checksum"` // SHA-256 hex
}

// Session represents the complete state of a review session.
// Stored as a single session.json file, updated in-place each round.
type Session struct {
	Version        int       `json:"version"`
	SessionID      string    `json:"session_id"`
	XReviewVersion string    `json:"xreview_version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Status         string    `json:"status"`
	Round          int       `json:"round"`
	CodexSessionID string    `json:"codex_session_id,omitempty"`
	CodexModel     string    `json:"codex_model"`
	Context        string    `json:"context"`
	Targets        []string  `json:"targets"`
	TargetMode     string    `json:"target_mode"`
	Findings       []Finding       `json:"findings"`
	FileSnapshots  []FileSnapshot  `json:"file_snapshots,omitempty"`
}

// Finding represents a single review finding.
type Finding struct {
	ID               string `json:"id"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	Status           string `json:"status"`
	File             string `json:"file"`
	Line             int    `json:"line"`
	Description      string `json:"description"`
	Suggestion       string `json:"suggestion"`
	CodeSnippet      string           `json:"code_snippet,omitempty"`
	VerificationNote string           `json:"verification_note,omitempty"`
	Trigger          string           `json:"trigger,omitempty"`
	CascadeImpact    []string         `json:"cascade_impact,omitempty"`
	FixAlternatives  []FixAlternative `json:"fix_alternatives,omitempty"`
	Confidence       int              `json:"confidence"`
	FixStrategy      string           `json:"fix_strategy"`
}

// FixAlternative represents one possible fix approach for a finding.
type FixAlternative struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Effort      string `json:"effort"` // minimal, moderate, large
	Recommended bool   `json:"recommended"`
}

// FindingSummary holds aggregated counts of finding statuses.
type FindingSummary struct {
	Total     int `json:"total"`
	Open      int `json:"open"`
	Fixed     int `json:"fixed"`
	Dismissed int `json:"dismissed"`
}

// Summarize computes a FindingSummary from the current findings.
func (s *Session) Summarize() FindingSummary {
	sum := FindingSummary{Total: len(s.Findings)}
	for _, f := range s.Findings {
		switch f.Status {
		case FindingOpen, FindingReopened:
			sum.Open++
		case FindingFixed:
			sum.Fixed++
		case FindingDismissed:
			sum.Dismissed++
		}
	}
	return sum
}

// CodexResponse represents the structured JSON output from codex.
type CodexResponse struct {
	Verdict  string         `json:"verdict"`
	Summary  string         `json:"summary"`
	Findings []CodexFinding `json:"findings"`
}

// CodexFinding is a single finding as returned by codex JSON output.
// Confidence is a pointer to distinguish "not provided" (nil) from "explicitly 0".
type CodexFinding struct {
	ID               string `json:"id"`
	Severity         string `json:"severity"`
	Category         string `json:"category"`
	File             string `json:"file"`
	Line             int    `json:"line"`
	Description      string `json:"description"`
	Suggestion       string `json:"suggestion"`
	CodeSnippet      string `json:"code_snippet,omitempty"`
	Status           string           `json:"status,omitempty"`
	VerificationNote string           `json:"verification_note,omitempty"`
	Trigger          string           `json:"trigger,omitempty"`
	CascadeImpact    []string         `json:"cascade_impact,omitempty"`
	FixAlternatives  []FixAlternative `json:"fix_alternatives,omitempty"`
	Confidence       *int             `json:"confidence"`
	FixStrategy      string           `json:"fix_strategy"`
}

// ConfidenceOrDefault returns the confidence value, or fallback if Codex did not provide one.
func (f *CodexFinding) ConfidenceOrDefault(fallback int) int {
	if f.Confidence != nil {
		return *f.Confidence
	}
	return fallback
}
