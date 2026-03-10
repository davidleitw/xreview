# Plan 4: Prompt Builder

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Assemble prompts for codex: first round (diff + file list), resume round (previous findings + updated files), and full rescan.

**Architecture:** `internal/prompt/` package. Template-based prompt assembly using Go `text/template`. Templates stored as Go string constants.

**Tech Stack:** Go stdlib (`text/template`, `strings`, `fmt`)

**Depends on:** Plan 1 (types), Plan 3 (collector types)

---

## Chunk 1: Prompt Templates + Builder

### File Structure

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `internal/prompt/templates.go` | Prompt template string constants |
| Create | `internal/prompt/builder.go` | Prompt assembly for all three modes |
| Create | `internal/prompt/builder_test.go` | Template rendering tests |

---

### Task 4.1: Prompt Templates

**Files:**
- Create: `internal/prompt/templates.go`

- [ ] **Step 1: Write templates.go**

Create `internal/prompt/templates.go`:

```go
package prompt

const firstRoundTemplate = `<CRITICAL_RULES>
1. PERFORM STATIC ANALYSIS ONLY. Do NOT execute or run the code.
2. Only report issues you can directly observe in the provided code.
   Do NOT speculate about issues in code you cannot see.
3. Every finding MUST reference a specific file and line number.
4. Focus on real bugs and security issues. Do NOT report trivial style preferences.
5. If you find no issues, set verdict to APPROVED with an empty findings array.
6. You are encouraged to read additional files in the repository if needed
   to understand the full context of the code being reviewed.
</CRITICAL_RULES>

You are a senior code reviewer. Analyze the following code changes for bugs,
security vulnerabilities, logic errors, and significant quality issues.

Context from the developer: {{.Context}}

===== FILES CHANGED =====

{{.FileList}}

===== DIFF =====

{{.Diff}}

===== END =====`

const resumeTemplate = `This is a follow-up review. You previously reviewed these files and
identified the findings listed below. The developer has made changes
and provided the following update:

Developer message: "{{.Message}}"

===== PREVIOUS FINDINGS =====

{{.PreviousFindings}}

===== UPDATED FILES =====

{{.UpdatedFiles}}

===== END OF FILES =====

For each previous finding, determine:
1. If claimed fixed: verify the fix is actually correct and complete.
2. If claimed false positive: evaluate whether the dismissal is reasonable.
3. If no update: re-evaluate against the current code.

Also check: did any of the changes introduce NEW issues?

New findings (not in the previous list) should have status "open" and no "id" field.`

const fullRescanTemplate = `<CRITICAL_RULES>
1. PERFORM STATIC ANALYSIS ONLY. Do NOT execute or run the code.
2. Only report issues you can directly observe in the provided code.
   Do NOT speculate about issues in code you cannot see.
3. Every finding MUST reference a specific file and line number.
4. Focus on real bugs and security issues. Do NOT report trivial style preferences.
5. If you find no issues, set verdict to APPROVED with an empty findings array.
6. You are encouraged to read additional files in the repository if needed
   to understand the full context of the code being reviewed.
</CRITICAL_RULES>

You are a senior code reviewer. This is a FULL RESCAN of previously reviewed files.
Analyze the code fresh for bugs, security vulnerabilities, logic errors, and
significant quality issues.

Context from the developer: {{.Context}}

The following finding IDs were identified in a previous review. If you find the same
issues still present, use the same ID. If an issue is no longer present, do not include it.

Previous finding IDs for reference:
{{.PreviousFindingIDs}}

===== FILES CHANGED =====

{{.FileList}}

===== DIFF =====

{{.Diff}}

===== END =====`
```

- [ ] **Step 2: Commit**

```bash
git add internal/prompt/templates.go
git commit -m "feat: add prompt template constants for all review modes"
```

---

### Task 4.2: Prompt Builder

**Files:**
- Create: `internal/prompt/builder.go`
- Test: `internal/prompt/builder_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/prompt/builder_test.go`:

```go
package prompt

import (
	"strings"
	"testing"

	"github.com/davidleitw/xreview/internal/session"
)

func TestBuildFirstRound(t *testing.T) {
	p, err := BuildFirstRound("Added auth system", "- src/auth.go (50 lines)\n", "diff --git a/src/auth.go...")
	if err != nil {
		t.Fatalf("BuildFirstRound: %v", err)
	}

	if !strings.Contains(p, "CRITICAL_RULES") {
		t.Error("missing CRITICAL_RULES")
	}
	if !strings.Contains(p, "Added auth system") {
		t.Error("missing context")
	}
	if !strings.Contains(p, "src/auth.go (50 lines)") {
		t.Error("missing file list")
	}
	if !strings.Contains(p, "diff --git") {
		t.Error("missing diff")
	}
}

func TestBuildResume(t *testing.T) {
	findings := []session.Finding{
		{
			ID:          "F001",
			Severity:    "high",
			Category:    "security",
			File:        "src/auth.go",
			Line:        42,
			Description: "JWT not checked",
		},
		{
			ID:          "F002",
			Severity:    "medium",
			Category:    "logic",
			File:        "src/mid.go",
			Line:        15,
			Description: "Error wrapping",
		},
	}

	files := "--- src/auth.go ---\n     1\tpackage main\n"

	p, err := BuildResume("Fixed F001", findings, files)
	if err != nil {
		t.Fatalf("BuildResume: %v", err)
	}

	if !strings.Contains(p, "follow-up review") {
		t.Error("missing follow-up header")
	}
	if !strings.Contains(p, "Fixed F001") {
		t.Error("missing message")
	}
	if !strings.Contains(p, "F001 [HIGH / security]") {
		t.Error("missing finding F001")
	}
	if !strings.Contains(p, "F002 [MEDIUM / logic]") {
		t.Error("missing finding F002")
	}
}

func TestBuildFullRescan(t *testing.T) {
	findings := []session.Finding{
		{ID: "F001"},
		{ID: "F002"},
	}

	p, err := BuildFullRescan("Re-review", findings, "- auth.go\n", "diff...")
	if err != nil {
		t.Fatalf("BuildFullRescan: %v", err)
	}

	if !strings.Contains(p, "FULL RESCAN") {
		t.Error("missing FULL RESCAN")
	}
	if !strings.Contains(p, "F001") {
		t.Error("missing F001 reference")
	}
	if !strings.Contains(p, "F002") {
		t.Error("missing F002 reference")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Write builder.go**

Create `internal/prompt/builder.go`:

```go
package prompt

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/davidleitw/xreview/internal/session"
)

// BuildFirstRound assembles the prompt for a new review.
func BuildFirstRound(context, fileList, diff string) (string, error) {
	tmpl, err := template.New("first").Parse(firstRoundTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"Context":  context,
		"FileList": fileList,
		"Diff":     diff,
	})
	return buf.String(), err
}

// BuildResume assembles the prompt for a resume/verify round.
func BuildResume(message string, previousFindings []session.Finding, updatedFiles string) (string, error) {
	tmpl, err := template.New("resume").Parse(resumeTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"Message":          message,
		"PreviousFindings": formatPreviousFindings(previousFindings),
		"UpdatedFiles":     updatedFiles,
	})
	return buf.String(), err
}

// BuildFullRescan assembles the prompt for a full rescan.
func BuildFullRescan(context string, previousFindings []session.Finding, fileList, diff string) (string, error) {
	tmpl, err := template.New("rescan").Parse(fullRescanTemplate)
	if err != nil {
		return "", err
	}

	var ids []string
	for _, f := range previousFindings {
		ids = append(ids, f.ID)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"Context":            context,
		"PreviousFindingIDs": strings.Join(ids, ", "),
		"FileList":           fileList,
		"Diff":               diff,
	})
	return buf.String(), err
}

func formatPreviousFindings(findings []session.Finding) string {
	var b strings.Builder
	for _, f := range findings {
		fmt.Fprintf(&b, "%s [%s / %s] %s:%d\n",
			f.ID,
			strings.ToUpper(f.Severity),
			f.Category,
			f.File,
			f.Line,
		)
		fmt.Fprintf(&b, "  %s\n\n", f.Description)
	}
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/davidleitw/xreview && go test ./internal/prompt/ -v`
Expected: All 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/prompt/
git commit -m "feat: add prompt builder for first round, resume, and full rescan"
```
