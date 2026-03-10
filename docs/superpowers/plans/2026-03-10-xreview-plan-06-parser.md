# Plan 6: Parser — Codex Output JSON Parsing

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Parse codex JSON stdout into structured findings. Handle clean JSON, code-fenced JSON, JSON with preamble, malformed JSON, and missing fields.

**Architecture:** `internal/parser/` package. `extract.go` handles raw text → clean JSON extraction. `parser.go` handles JSON → CodexResponse struct with validation.

**Tech Stack:** Go stdlib (`encoding/json`, `strings`, `regexp`)

**Depends on:** Plan 1 (types — CodexResponse, CodexFinding)

---

## Chunk 1: JSON Extraction + Parsing + Validation

### File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `internal/parser/extract.go` | Extract JSON from raw codex output (strip fences, preamble) |
| Create | `internal/parser/extract_test.go` | Extraction tests with various formats |
| Create | `internal/parser/parser.go` | Parse extracted JSON into CodexResponse, validate fields |
| Create | `internal/parser/parser_test.go` | Parsing + validation tests |
| Create | `test/fixtures/codex-output/clean-json.txt` | Fixture: clean JSON |
| Create | `test/fixtures/codex-output/json-in-code-fence.txt` | Fixture: fenced JSON |
| Create | `test/fixtures/codex-output/json-with-preamble.txt` | Fixture: preamble + JSON |
| Create | `test/fixtures/codex-output/malformed-json.txt` | Fixture: invalid JSON |
| Create | `test/fixtures/codex-output/empty-findings.txt` | Fixture: empty findings |
| Create | `test/fixtures/codex-output/missing-fields.txt` | Fixture: missing required fields |

---

### Task 6.1: Test Fixtures

**Files:**
- Create: `test/fixtures/codex-output/*.txt`

- [ ] **Step 1: Create fixture files**

Create `test/fixtures/codex-output/clean-json.txt`:

```json
{
  "verdict": "REVISE",
  "summary": "Found 2 issues",
  "findings": [
    {
      "id": "F001",
      "severity": "high",
      "category": "security",
      "file": "src/auth.go",
      "line": 42,
      "description": "JWT not checked for expiration",
      "suggestion": "Add exp validation"
    },
    {
      "id": "F002",
      "severity": "medium",
      "category": "logic",
      "file": "src/mid.go",
      "line": 15,
      "description": "Error without context",
      "suggestion": "Wrap error"
    }
  ]
}
```

Create `test/fixtures/codex-output/json-in-code-fence.txt`:

````
Here is my analysis:

```json
{
  "verdict": "REVISE",
  "summary": "Found 1 issue",
  "findings": [
    {
      "id": "F001",
      "severity": "high",
      "category": "security",
      "file": "src/auth.go",
      "line": 42,
      "description": "JWT not checked",
      "suggestion": "Add validation"
    }
  ]
}
```
````

Create `test/fixtures/codex-output/json-with-preamble.txt`:

```
I have reviewed the code and found the following issues:

{"verdict":"REVISE","summary":"Found 1 issue","findings":[{"id":"F001","severity":"high","category":"security","file":"src/auth.go","line":42,"description":"JWT issue","suggestion":"Fix it"}]}
```

Create `test/fixtures/codex-output/malformed-json.txt`:

```
This is not valid JSON at all.
No structured output here.
```

Create `test/fixtures/codex-output/empty-findings.txt`:

```json
{
  "verdict": "APPROVED",
  "summary": "No issues found",
  "findings": []
}
```

Create `test/fixtures/codex-output/missing-fields.txt`:

```json
{
  "verdict": "REVISE",
  "summary": "Issues found",
  "findings": [
    {
      "id": "F001",
      "severity": "high",
      "description": "Valid finding",
      "suggestion": "Fix it"
    },
    {
      "description": "Missing severity"
    }
  ]
}
```

- [ ] **Step 2: Commit**

```bash
mkdir -p test/fixtures/codex-output
git add test/fixtures/codex-output/
git commit -m "test: add codex output fixture files for parser tests"
```

---

### Task 6.2: JSON Extraction

**Files:**
- Create: `internal/parser/extract.go`
- Test: `internal/parser/extract_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/parser/extract_test.go`:

```go
package parser

import (
	"os"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../test/fixtures/codex-output/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func TestExtractJSON_CleanJSON(t *testing.T) {
	raw := readFixture(t, "clean-json.txt")
	result, err := ExtractJSON(raw)
	if err != nil {
		t.Fatalf("ExtractJSON: %v", err)
	}
	if result[0] != '{' {
		t.Errorf("should start with '{', got %q", string(result[0]))
	}
}

func TestExtractJSON_CodeFence(t *testing.T) {
	raw := readFixture(t, "json-in-code-fence.txt")
	result, err := ExtractJSON(raw)
	if err != nil {
		t.Fatalf("ExtractJSON: %v", err)
	}
	if result[0] != '{' {
		t.Errorf("should start with '{', got %q", string(result[0]))
	}
}

func TestExtractJSON_Preamble(t *testing.T) {
	raw := readFixture(t, "json-with-preamble.txt")
	result, err := ExtractJSON(raw)
	if err != nil {
		t.Fatalf("ExtractJSON: %v", err)
	}
	if result[0] != '{' {
		t.Errorf("should start with '{'")
	}
}

func TestExtractJSON_Malformed(t *testing.T) {
	raw := readFixture(t, "malformed-json.txt")
	_, err := ExtractJSON(raw)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/parser/ -v -run TestExtract`
Expected: FAIL — function not defined

- [ ] **Step 3: Write extract.go**

Create `internal/parser/extract.go`:

```go
package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var codeFenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.*?)\\n```")

// ExtractJSON attempts to extract valid JSON from raw codex output.
// Tries in order: direct parse, strip code fences, find JSON object in text.
func ExtractJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)

	// Try 1: Direct parse
	if json.Valid([]byte(raw)) {
		return raw, nil
	}

	// Try 2: Strip markdown code fences
	matches := codeFenceRe.FindStringSubmatch(raw)
	if len(matches) > 1 {
		candidate := strings.TrimSpace(matches[1])
		if json.Valid([]byte(candidate)) {
			return candidate, nil
		}
	}

	// Try 3: Find first '{' and last '}' — extract JSON object
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		candidate := raw[start : end+1]
		if json.Valid([]byte(candidate)) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not extract valid JSON from codex output")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/parser/ -v -run TestExtract`
Expected: All 4 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/parser/extract.go internal/parser/extract_test.go
git commit -m "feat: add JSON extraction from raw codex output with fallbacks"
```

---

### Task 6.3: JSON Parser with Validation

**Files:**
- Create: `internal/parser/parser.go`
- Test: `internal/parser/parser_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/parser/parser_test.go`:

```go
package parser

import (
	"testing"
)

func TestParse_CleanJSON(t *testing.T) {
	raw := readFixture(t, "clean-json.txt")
	resp, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if resp.Verdict != "REVISE" {
		t.Errorf("verdict: got %q, want REVISE", resp.Verdict)
	}
	if len(resp.Findings) != 2 {
		t.Errorf("findings: got %d, want 2", len(resp.Findings))
	}
	if resp.Findings[0].Severity != "high" {
		t.Errorf("first finding severity: got %q", resp.Findings[0].Severity)
	}
}

func TestParse_EmptyFindings(t *testing.T) {
	raw := readFixture(t, "empty-findings.txt")
	resp, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if resp.Verdict != "APPROVED" {
		t.Errorf("verdict: got %q, want APPROVED", resp.Verdict)
	}
	if len(resp.Findings) != 0 {
		t.Errorf("findings: got %d, want 0", len(resp.Findings))
	}
}

func TestParse_CodeFence(t *testing.T) {
	raw := readFixture(t, "json-in-code-fence.txt")
	resp, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(resp.Findings) != 1 {
		t.Errorf("findings: got %d, want 1", len(resp.Findings))
	}
}

func TestParse_Malformed(t *testing.T) {
	raw := readFixture(t, "malformed-json.txt")
	_, err := Parse(raw)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParse_MissingFields(t *testing.T) {
	raw := readFixture(t, "missing-fields.txt")
	resp, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Second finding is missing severity — should be skipped
	if len(resp.Findings) != 1 {
		t.Errorf("findings: got %d, want 1 (invalid should be skipped)", len(resp.Findings))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/parser/ -v -run "TestParse_"`
Expected: FAIL — Parse not defined

- [ ] **Step 3: Write parser.go**

Create `internal/parser/parser.go`:

```go
package parser

import (
	"encoding/json"
	"fmt"

	"github.com/davidleitw/xreview/internal/session"
)

// Parse extracts and parses codex output into a CodexResponse.
// Skips findings that are missing required fields (severity, description).
func Parse(raw string) (*session.CodexResponse, error) {
	jsonStr, err := ExtractJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse failure: %w", err)
	}

	var resp session.CodexResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("JSON unmarshal: %w", err)
	}

	// Filter out findings with missing required fields
	valid := make([]session.CodexFinding, 0, len(resp.Findings))
	for _, f := range resp.Findings {
		if f.Severity == "" || f.Description == "" {
			continue
		}
		valid = append(valid, f)
	}
	resp.Findings = valid

	return &resp, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/parser/ -v`
Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/parser/parser.go internal/parser/parser_test.go
git commit -m "feat: add codex output parser with validation and missing-field filtering"
```
