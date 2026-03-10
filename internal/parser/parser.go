package parser

import (
	"encoding/json"
	"fmt"

	"github.com/davidleitw/xreview/internal/session"
)

// Parser parses codex stdout into structured findings.
type Parser interface {
	Parse(stdout string) (*session.CodexResponse, error)
}

type parser struct{}

// NewParser creates a Parser.
func NewParser() Parser {
	return &parser{}
}

func (p *parser) Parse(stdout string) (*session.CodexResponse, error) {
	cleaned, err := ExtractJSON(stdout)
	if err != nil {
		return nil, fmt.Errorf("extract JSON: %w", err)
	}

	var resp session.CodexResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal codex response: %w", err)
	}

	return &resp, nil
}
