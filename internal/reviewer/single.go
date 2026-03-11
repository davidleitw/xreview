package reviewer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/davidleitw/xreview/internal/codex"
	"github.com/davidleitw/xreview/internal/collector"
	"github.com/davidleitw/xreview/internal/config"
	"github.com/davidleitw/xreview/internal/parser"
	"github.com/davidleitw/xreview/internal/prompt"
	"github.com/davidleitw/xreview/internal/schema"
	"github.com/davidleitw/xreview/internal/session"
)

// SingleReviewer uses a single codex call for review.
type SingleReviewer struct {
	runner    codex.Runner
	builder   prompt.Builder
	parser    parser.Parser
	sessions  session.Manager
	collector collector.Collector
	cfg       *config.Config
}

// NewSingleReviewer creates a SingleReviewer with the given dependencies.
func NewSingleReviewer(
	runner codex.Runner,
	builder prompt.Builder,
	parser parser.Parser,
	sessions session.Manager,
	collector collector.Collector,
	cfg *config.Config,
) *SingleReviewer {
	return &SingleReviewer{
		runner:    runner,
		builder:   builder,
		parser:    parser,
		sessions:  sessions,
		collector: collector,
		cfg:       cfg,
	}
}

func (r *SingleReviewer) Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	// 1. Create session
	sess, err := r.sessions.Create(req.Targets, req.TargetMode, req.Context, r.cfg)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// 2. Collect files
	files, err := r.collector.Collect(ctx, req.Targets, req.TargetMode)
	if err != nil {
		return nil, err
	}

	// 3. Build prompt
	fileList, diff := formatFilesForPrompt(files)
	promptStr, err := r.builder.BuildFirstRound(prompt.FirstRoundInput{
		Context:  req.Context,
		FileList: fileList,
		Diff:     diff,
	})
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	// 4. Write temp schema
	schemaPath, cleanup, err := schema.WriteTempSchema()
	if err != nil {
		return nil, fmt.Errorf("write schema: %w", err)
	}
	defer cleanup()

	// 5. Call codex
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = time.Duration(r.cfg.DefaultTimeout) * time.Second
	}

	execResult, err := r.runner.Exec(ctx, codex.ExecRequest{
		Model:      r.cfg.CodexModel,
		Prompt:     promptStr,
		SchemaPath: schemaPath,
		Timeout:    timeout,
	})
	if err != nil {
		return nil, err
	}

	// 6. Parse response
	codexResp, err := r.parser.Parse(execResult.Stdout)
	if err != nil {
		return nil, fmt.Errorf("parse codex output: %w", err)
	}

	// 7. Update session
	sess.Status = session.StatusInReview
	sess.Round = 1
	sess.CodexSessionID = execResult.CodexSessionID
	sess.Findings = codexFindingsToFindings(codexResp.Findings)
	if err := r.sessions.Update(sess); err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	summary := sess.Summarize()
	return &ReviewResult{
		SessionID: sess.SessionID,
		Round:     sess.Round,
		Verdict:   codexResp.Verdict,
		Findings:  sess.Findings,
		Summary:   summary,
	}, nil
}

func (r *SingleReviewer) Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	// 1. Load session
	sess, err := r.sessions.Load(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}

	// 2. Collect files from original session targets
	files, err := r.collector.Collect(ctx, sess.Targets, sess.TargetMode)
	if err != nil {
		return nil, err
	}

	// 3. Collect extra files if provided
	var additionalContent string
	if len(req.ExtraTargets) > 0 || req.ExtraTargetMode == "git-uncommitted" {
		extraFiles, err := r.collector.Collect(ctx, req.ExtraTargets, req.ExtraTargetMode)
		if err != nil {
			return nil, err
		}
		_, additionalContent = formatFilesForPrompt(extraFiles)
	}

	// 4. Build resume prompt
	_, updatedFiles := formatFilesForPrompt(files)
	promptStr, err := r.builder.BuildResume(prompt.ResumeInput{
		Message:          req.Message,
		PreviousFindings: r.builder.FormatFindingsForPrompt(sess.Findings),
		UpdatedFiles:     updatedFiles,
		AdditionalFiles:  additionalContent,
	})
	if err != nil {
		return nil, fmt.Errorf("build resume prompt: %w", err)
	}

	// 5. Write temp schema
	schemaPath, cleanup, err := schema.WriteTempSchema()
	if err != nil {
		return nil, fmt.Errorf("write schema: %w", err)
	}
	defer cleanup()

	// 6. Determine resume vs fresh
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = time.Duration(r.cfg.DefaultTimeout) * time.Second
	}

	execReq := codex.ExecRequest{
		Model:      r.cfg.CodexModel,
		Prompt:     promptStr,
		SchemaPath: schemaPath,
		Timeout:    timeout,
	}
	if codex.ShouldResume(sess, req.FullRescan) {
		execReq.ResumeSessionID = sess.CodexSessionID
	}

	execResult, err := r.runner.Exec(ctx, execReq)
	if err != nil {
		return nil, err
	}

	// 7. Parse response
	codexResp, err := r.parser.Parse(execResult.Stdout)
	if err != nil {
		return nil, fmt.Errorf("parse codex output: %w", err)
	}

	// 8. Merge findings
	sess.Round++
	sess.Status = session.StatusVerifying
	if execResult.CodexSessionID != "" {
		sess.CodexSessionID = execResult.CodexSessionID
	}
	sess.Findings = mergeFindings(sess.Findings, codexResp.Findings)

	if err := r.sessions.Update(sess); err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	summary := sess.Summarize()
	return &VerifyResult{
		SessionID: sess.SessionID,
		Round:     sess.Round,
		Verdict:   codexResp.Verdict,
		Findings:  sess.Findings,
		Summary:   summary,
	}, nil
}

// formatFilesForPrompt creates a file list and combined content from collected files.
func formatFilesForPrompt(files []collector.FileContent) (fileList string, content string) {
	var listBuf, contentBuf strings.Builder
	for _, f := range files {
		fmt.Fprintf(&listBuf, "%s (%d lines)\n", f.Path, f.Lines)
		fmt.Fprintf(&contentBuf, "--- %s ---\n%s\n", f.Path, f.Content)
	}
	return listBuf.String(), contentBuf.String()
}

// codexFindingsToFindings converts codex findings to session findings.
func codexFindingsToFindings(cf []session.CodexFinding) []session.Finding {
	findings := make([]session.Finding, len(cf))
	for i, f := range cf {
		status := f.Status
		if status == "" {
			status = session.FindingOpen
		}
		findings[i] = session.Finding{
			ID:               f.ID,
			Severity:         f.Severity,
			Category:         f.Category,
			Status:           status,
			File:             f.File,
			Line:             f.Line,
			Description:      f.Description,
			Suggestion:       f.Suggestion,
			CodeSnippet:      f.CodeSnippet,
			VerificationNote: f.VerificationNote,
		}
	}
	return findings
}

// mergeFindings merges new codex findings into existing session findings.
// Existing findings are updated by ID; new findings are appended.
func mergeFindings(existing []session.Finding, incoming []session.CodexFinding) []session.Finding {
	byID := make(map[string]int, len(existing))
	for i, f := range existing {
		byID[f.ID] = i
	}

	for _, cf := range incoming {
		status := cf.Status
		if status == "" {
			status = session.FindingOpen
		}
		if idx, ok := byID[cf.ID]; ok {
			// Update existing finding
			existing[idx].Status = status
			existing[idx].VerificationNote = cf.VerificationNote
			if cf.Description != "" {
				existing[idx].Description = cf.Description
			}
			if cf.Suggestion != "" {
				existing[idx].Suggestion = cf.Suggestion
			}
		} else {
			// New finding
			existing = append(existing, session.Finding{
				ID:               cf.ID,
				Severity:         cf.Severity,
				Category:         cf.Category,
				Status:           status,
				File:             cf.File,
				Line:             cf.Line,
				Description:      cf.Description,
				Suggestion:       cf.Suggestion,
				CodeSnippet:      cf.CodeSnippet,
				VerificationNote: cf.VerificationNote,
			})
		}
	}

	return existing
}
