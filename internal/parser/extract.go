package parser

import (
	"fmt"
	"regexp"
	"strings"
)

var uuidRegex = regexp.MustCompile(
	`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`,
)

// ExtractJSON attempts to extract clean JSON from raw codex output.
// If the output is wrapped in code fences, they are stripped.
func ExtractJSON(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("empty output")
	}

	// Try direct parse first (--output-schema should produce clean JSON)
	if strings.HasPrefix(trimmed, "{") {
		return trimmed, nil
	}

	// Fallback: strip markdown code fences
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		var jsonLines []string
		inside := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inside = !inside
				continue
			}
			if inside {
				jsonLines = append(jsonLines, line)
			}
		}
		result := strings.Join(jsonLines, "\n")
		if strings.TrimSpace(result) != "" {
			return result, nil
		}
	}

	return "", fmt.Errorf("could not extract JSON from output")
}

// ExtractCodexSessionID extracts a UUID session ID from codex stderr output.
func ExtractCodexSessionID(stderr string) (string, error) {
	match := uuidRegex.FindString(stderr)
	if match == "" {
		return "", fmt.Errorf("no session ID found in codex stderr")
	}
	return match, nil
}
