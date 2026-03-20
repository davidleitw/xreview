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
	workdir   string
}

// NewSingleReviewer creates a SingleReviewer with the given dependencies.
func NewSingleReviewer(
	runner codex.Runner,
	builder prompt.Builder,
	parser parser.Parser,
	sessions session.Manager,
	collector collector.Collector,
	cfg *config.Config,
	workdir string,
) *SingleReviewer {
	return &SingleReviewer{
		runner:    runner,
		builder:   builder,
		parser:    parser,
		sessions:  sessions,
		collector: collector,
		cfg:       cfg,
		workdir:   workdir,
	}
}

func (r *SingleReviewer) Review(ctx context.Context, req ReviewRequest) (*ReviewResult, error) {
	// 1. Create session
	sess, err := r.sessions.Create(req.Targets, req.TargetMode, req.Context, r.cfg)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// 2. Collect file metadata (for file list summary only)
	files, err := r.collector.Collect(ctx, req.Targets, req.TargetMode)
	if err != nil {
		return nil, err
	}

	// 3. Build prompt (instruction-only, no file content)
	promptStr, err := r.builder.BuildFirstRound(prompt.FirstRoundInput{
		Context:     req.Context,
		FetchMethod: buildFetchMethod(req.Targets, req.TargetMode),
		FileList:    buildFileListSummary(files),
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

	// 8. Snapshot file checksums for change detection in subsequent rounds
	snapshots, snapErr := collector.Snapshot(req.Targets, req.TargetMode, r.workdir, r.cfg.IgnorePatterns)
	if snapErr == nil {
		sess.FileSnapshots = snapshots
	}

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

	// 2. Detect file changes since last round (before Collect, so deleted files are caught)
	currentSnapshots, _ := collector.Snapshot(sess.Targets, sess.TargetMode, r.workdir, r.cfg.IgnorePatterns)
	var changedFiles []prompt.FileChange
	if len(sess.FileSnapshots) > 0 && len(currentSnapshots) > 0 {
		diffs := collector.DiffSnapshots(sess.FileSnapshots, currentSnapshots)
		for _, d := range diffs {
			changedFiles = append(changedFiles, prompt.FileChange{
				Path:   d.Path,
				Status: d.Status,
			})
		}
	}

	// 3. Collect file metadata for summary (best-effort: files may have been deleted)
	files, _ := r.collector.Collect(ctx, sess.Targets, sess.TargetMode)

	// 4. Build resume prompt (instruction-only, Codex reads files itself)
	promptStr, err := r.builder.BuildResume(prompt.ResumeInput{
		Message:          req.Message,
		PreviousFindings: r.builder.FormatFindingsForPrompt(sess.Findings),
		FetchMethod:      buildFetchMethod(sess.Targets, sess.TargetMode),
		FileList:         buildFileListSummary(files),
		ChangedFiles:     changedFiles,
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

	// 9. Update file snapshots for next round
	if len(currentSnapshots) > 0 {
		sess.FileSnapshots = currentSnapshots
	}

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

// buildFetchMethod constructs the instruction for Codex to get the code to review.
// Two distinct modes:
//   - "git-uncommitted": Codex runs git diff commands to see uncommitted changes
//   - "files": Codex reads the specified files directly (no git involved —
//     supports use cases like reviewing a single file's quality or tracing
//     a flow across multiple files described by --context)
func buildFetchMethod(targets []string, targetMode string) string {
	switch targetMode {
	case "git-uncommitted":
		return "Run these commands to see the uncommitted changes:\n" +
			"  git diff          # unstaged changes\n" +
			"  git diff --cached # staged changes\n" +
			"  git ls-files --others --exclude-standard  # untracked files (read their content too)"
	case "files":
		var b strings.Builder
		b.WriteString("Read the following files in full and review them:\n")
		for _, t := range targets {
			fmt.Fprintf(&b, "  - %s\n", t)
		}
		b.WriteString("\nThese are NOT necessarily git changes — you are reviewing the files themselves.\n")
		b.WriteString("Use the developer's --context description to understand what to focus on.")
		return b.String()
	default:
		return "git diff HEAD"
	}
}

// buildFileListSummary creates a brief summary of file names for the prompt.
func buildFileListSummary(files []collector.FileContent) string {
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "%s (%d lines)\n", f.Path, f.Lines)
	}
	return b.String()
}

// codexFindingsToFindings converts codex findings to session findings.
func codexFindingsToFindings(cf []session.CodexFinding) []session.Finding {
	findings := make([]session.Finding, len(cf))
	for i, f := range cf {
		status := f.Status
		if status == "" {
			status = session.FindingOpen
		}
		fixStrategy := f.FixStrategy
		if fixStrategy == "" {
			fixStrategy = "ask"
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
			Trigger:          f.Trigger,
			CascadeImpact:    f.CascadeImpact,
			FixAlternatives:  f.FixAlternatives,
			Confidence:       f.ConfidenceOrDefault(0),
			FixStrategy:      fixStrategy,
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
		fixStrategy := cf.FixStrategy
		if fixStrategy == "" {
			fixStrategy = "ask"
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
			// Preserve enriched data from earlier rounds when verification returns sparse findings
			if cf.Trigger != "" {
				existing[idx].Trigger = cf.Trigger
			}
			if len(cf.CascadeImpact) > 0 {
				existing[idx].CascadeImpact = cf.CascadeImpact
			}
			if len(cf.FixAlternatives) > 0 {
				existing[idx].FixAlternatives = cf.FixAlternatives
			}
			if cf.Confidence != nil {
				existing[idx].Confidence = *cf.Confidence
			}
			existing[idx].FixStrategy = fixStrategy
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
				Trigger:          cf.Trigger,
				CascadeImpact:    cf.CascadeImpact,
				FixAlternatives:  cf.FixAlternatives,
				Confidence:       cf.ConfidenceOrDefault(0),
				FixStrategy:      fixStrategy,
			})
		}
	}

	return existing
}
