package prompt

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/davidleitw/xreview/internal/session"
)

// FirstRoundInput holds the data for building a first-round prompt.
type FirstRoundInput struct {
	Context  string
	FileList string
	Diff     string
}

// ResumeInput holds the data for building a resume-round prompt.
type ResumeInput struct {
	Message          string
	PreviousFindings string
	UpdatedFiles     string
}

// Builder assembles prompts for codex.
type Builder interface {
	BuildFirstRound(input FirstRoundInput) (string, error)
	BuildResume(input ResumeInput) (string, error)
	// FormatFindingsForPrompt formats findings for inclusion in a resume prompt.
	FormatFindingsForPrompt(findings []session.Finding) string
}

type builder struct {
	firstRound *template.Template
	resume     *template.Template
}

// NewBuilder creates a Builder with the default prompt templates.
func NewBuilder() (Builder, error) {
	fr, err := template.New("first-round").Parse(FirstRoundTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse first-round template: %w", err)
	}
	rs, err := template.New("resume").Parse(ResumeTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse resume template: %w", err)
	}
	return &builder{firstRound: fr, resume: rs}, nil
}

func (b *builder) BuildFirstRound(input FirstRoundInput) (string, error) {
	var buf bytes.Buffer
	if err := b.firstRound.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("execute first-round template: %w", err)
	}
	return buf.String(), nil
}

func (b *builder) BuildResume(input ResumeInput) (string, error) {
	var buf bytes.Buffer
	if err := b.resume.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("execute resume template: %w", err)
	}
	return buf.String(), nil
}

func (b *builder) FormatFindingsForPrompt(findings []session.Finding) string {
	// TODO: implement — format findings as human-readable text for prompt
	return ""
}
