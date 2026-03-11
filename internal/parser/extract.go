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
// As a last resort, it scans for the first '{' and extracts a brace-matched object.
func ExtractJSON(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("empty output")
	}

	// Try direct parse first (--output-schema should produce clean JSON)
	if strings.HasPrefix(trimmed, "{") {
		return trimmed, nil
	}

	// Fallback 1: strip markdown code fences
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

	// Fallback 2: find the first '{' and extract the outermost JSON object
	// by matching braces. This handles cases where codex wraps JSON in prose.
	if idx := strings.Index(trimmed, "{"); idx >= 0 {
		candidate := trimmed[idx:]
		if end := findClosingBrace(candidate); end > 0 {
			return candidate[:end+1], nil
		}
	}

	return "", fmt.Errorf("could not extract JSON from output")
}

// findClosingBrace finds the index of the closing '}' that matches the
// opening '{' at position 0, accounting for nesting and JSON strings.
// Returns -1 if no matching brace is found.
func findClosingBrace(s string) int {
	depth := 0
	inString := false
	escaped := false
	for i, ch := range s {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// ExtractCodexSessionID extracts a UUID session ID from codex stderr output.
func ExtractCodexSessionID(stderr string) (string, error) {
	match := uuidRegex.FindString(stderr)
	if match == "" {
		return "", fmt.Errorf("no session ID found in codex stderr")
	}
	return match, nil
}
