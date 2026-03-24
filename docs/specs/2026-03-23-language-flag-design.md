# `--language` Flag Design Spec

**Date:** 2026-03-23
**Status:** Review
**Scope:** Add `--language` flag to `xreview review`, first wave supports C++

---

## 1. Goal

Enable language-specific review guidelines to be injected into Codex prompts via `--language <lang>`. When a supported language is specified, xreview appends embedded language-specific review content to the prompt. When no language is specified (or the language is unsupported), behavior is identical to today — Codex reviews based on its own domain knowledge.

**Non-goals:**
- Taste review / `review-taste` command (future, separate feature)
- Auto-detection inside xreview binary (detection is the skill's responsibility)
- Language-specific output schema changes
- Per-project language config in `.xreview/config.json`

---

## 2. Supported Languages

First wave: **`cpp`** (C++)

The supported language registry is an exported Go map in the `prompt` package:

```go
// internal/prompt/language_registry.go

// SupportedLanguages maps language keys to display names.
var SupportedLanguages = map[string]string{
    "cpp": "C++",
}

// SupportedLanguageList returns a comma-separated list of supported keys for error messages.
func SupportedLanguageList() string { ... }
```

The `prompt` package owns the registry. The `languages` sub-package (`internal/prompt/languages/`) only holds the embedded `.md` files via `//go:embed`. The `prompt` package imports `languages` internally to load file content.

Adding a new language = adding the `.md` file in `languages/` + one map entry in `language_registry.go` + one row in SKILL.md table. No other code changes.

---

## 3. Data Flow

```
Skill (SKILL.md)
  │  Reads file extensions of review targets
  │  Checks supported language list (hardcoded in SKILL.md)
  │  If match → adds --language <lang>
  │  If no match → omits --language (current behavior)
  ▼
xreview CLI (cmd_review.go)
  │  Parses --language flag
  │  Validates value against supportedLanguages map
  │  Unsupported value → error exit
  │  Passes language to ReviewRequest
  ▼
Reviewer (single.go)
  │  Stores language in session
  │  Passes language to prompt.Builder
  ▼
Prompt Builder (builder.go)
  │  If language != "" → loads embedded .md file
  │  Appends language section (wrapper + content) to prompt
  ▼
Codex
  │  Receives prompt with language-specific guidelines appended
  │  Reviews code against both general rules AND language rules
  ▼
Output (formatter/xml.go)
  │  Includes language="cpp" attribute on <xreview-result>
  ▼
Skill
    Sees language attribute, knows this was a language-aware review
```

---

## 4. CLI Changes

### 4.1 New Flag

In `cmd/xreview/cmd_review.go`:

```go
var language string

cmd.Flags().StringVar(&language, "language", "", "Language-specific review guidelines (e.g. cpp)")
```

### 4.2 Validation

In `PreRunE`, after existing validation:

```go
if language != "" {
    if _, ok := prompt.SupportedLanguages[language]; !ok {
        return fmt.Errorf("unsupported language %q; supported: %s",
            language, prompt.SupportedLanguageList())
    }
}
```

Validation lives in `PreRunE` so unsupported values fail fast before any session/codex work.

### 4.3 Flag Propagation

`--language` is passed through `ReviewRequest.Language` to the reviewer. On resume (`--session`), the language is read from the persisted session — the flag is NOT required on resume calls. If the user provides `--language` on a resume call, it is ignored (session language is authoritative).

---

## 5. Prompt Changes

### 5.1 Embedded Language Files

```
internal/prompt/
├── builder.go
├── templates.go
└── languages/
    ├── embed.go       # //go:embed *.md
    └── cpp.md         # C++ review guidelines (content TBD, user provides)
```

`embed.go`:
```go
package languages

import "embed"

//go:embed *.md
var FS embed.FS
```

### 5.2 Language Section Wrapper

A fixed Go template wraps the language content. Defined in `templates.go`:

```go
const LanguageSectionTemplate = `

===== LANGUAGE-SPECIFIC REVIEW GUIDELINES ({{.DisplayName}}) =====

The files under review are primarily written in {{.DisplayName}}.
In addition to the general review rules above, you MUST also apply the
following language-specific guidelines. These carry the same weight
as CRITICAL_RULES — violations should be reported as findings.

{{.Content}}

===== END LANGUAGE-SPECIFIC GUIDELINES =====`
```

Key design decisions:
- Uses `=====` delimiters consistent with existing template sections (e.g., `===== HOW TO GET THE CODE =====`)
- Explicit instruction: "same weight as CRITICAL_RULES"
- Placed AFTER the main template content (appended, not inserted mid-template)
- For `ResumeTemplate`, this means after the JSON schema instructions at the end — Codex processes the full prompt so position is not critical, and appending keeps the logic uniform across both templates

### 5.3 Builder Changes

`FirstRoundInput` and `ResumeInput` both get a new field:

```go
type FirstRoundInput struct {
    Context     string
    FetchMethod string
    FileList    string
    Language    string // language key, e.g. "cpp". Empty = no language section.
}
```

In `BuildFirstRound()` and `BuildResume()`:

```go
func (b *builder) BuildFirstRound(input FirstRoundInput) (string, error) {
    var buf bytes.Buffer
    if err := b.firstRound.Execute(&buf, input); err != nil {
        return "", fmt.Errorf("execute first-round template: %w", err)
    }
    // Append language section if specified
    if input.Language != "" {
        section, err := buildLanguageSection(input.Language)
        if err != nil {
            return "", err
        }
        buf.WriteString(section)
    }
    return buf.String(), nil
}
```

`buildLanguageSection()` loads the `.md` from the embedded FS, renders the wrapper template, and returns the string. This is a standalone function, not tied to the template engine — keeping it decoupled from the main templates.

### 5.4 Resume Behavior

The resume prompt also appends the language section. The language value comes from the session (not from CLI flags), ensuring consistency across rounds:

```go
// In BuildResume:
if input.Language != "" {
    section, err := buildLanguageSection(input.Language)
    if err != nil {
        return "", err
    }
    buf.WriteString(section)
}
```

---

## 6. Session Changes

### 6.1 Session Struct

Add `Language` field to `session.Session`:

```go
type Session struct {
    // ... existing fields ...
    Language string `json:"language,omitempty"` // language key, e.g. "cpp"
}
```

Using `omitempty` — sessions without language produce identical JSON to today. This is a backward-compatible additive field, so **no session version bump** is needed. Old sessions missing the field will deserialize with `Language: ""`, which means "no language" — correct behavior.

### 6.2 Session Creation — No Interface Change

The `session.Manager` interface is NOT modified. `Create()` signature stays the same. Instead, the reviewer sets `Language` on the returned session before calling `Update()`:

```go
// In single.go Review():
sess, err := r.sessions.Create(req.Targets, req.TargetMode, req.Context, r.cfg)
// ... (existing code) ...
sess.Language = req.Language  // set language before Update()
// ... (existing session updates: Status, Round, CodexSessionID, Findings, etc.) ...
if err := r.sessions.Update(sess); err != nil { ... }
```

This avoids changing the `Manager` interface, which would ripple into all implementations and test mocks.

### 6.3 Reviewer Passes Language to Builder

In `single.go`, `Review()` reads `req.Language` and passes it to the prompt builder:

```go
promptStr, err := r.builder.BuildFirstRound(prompt.FirstRoundInput{
    Context:     req.Context,
    FetchMethod: buildFetchMethod(req.Targets, req.TargetMode),
    FileList:    buildFileListSummary(files),
    Language:    req.Language,
})
```

In `Verify()`, language comes from the loaded session and is passed to both the prompt builder and the result:

```go
promptStr, err := r.builder.BuildResume(prompt.ResumeInput{
    // ... existing fields ...
    Language: sess.Language,
})
// ... after codex call and parsing ...
return &VerifyResult{
    SessionID: sess.SessionID,
    Round:     sess.Round,
    Verdict:   codexResp.Verdict,
    Findings:  sess.Findings,
    Summary:   summary,
    Language:  sess.Language, // propagate to caller for XML output
}, nil
```

Similarly, `Review()` populates `ReviewResult.Language` from `req.Language`.

---

## 7. XML Output Changes

Add `language` attribute to `<xreview-result>` when language is specified:

```xml
<!-- With language -->
<xreview-result status="success" action="review" session="abc-123" round="1" language="cpp">

<!-- Without language (unchanged from today) -->
<xreview-result status="success" action="review" session="abc-123" round="1">
```

This requires `FormatReviewResult()` to accept a language parameter (appended as the last positional parameter to minimize churn on existing call sites — callers without language pass `""`). The attribute is only emitted when non-empty, preserving backward compatibility for tools parsing the XML.

The `ReviewResult` and `VerifyResult` structs both get a `Language string` field. `ReviewResult.Language` is set from `req.Language`; `VerifyResult.Language` is set from `sess.Language`. Both call sites in `cmd_review.go` (new review line ~120, verify line ~93) pass `result.Language` to `FormatReviewResult()`.

---

## 8. Skill Changes (SKILL.md)

Add a supported languages table near the top of SKILL.md (after the `<CRITICAL>` block), and update Step 2 command examples.

```markdown
## Supported Languages for --language

| Key   | Language |
|-------|----------|
| `cpp` | C++      |

If review targets are written in a supported language, add `--language <key>`.
If unsure or mixed languages, omit `--language` — xreview falls back to general-purpose review.
Only use keys from the table above.
```

Step 2 commands become:

```
xreview review --files <paths> --context "<context>" [--language <key>]
xreview review --git-uncommitted --context "<context>" [--language <key>]
```

No other skill steps change. The language flag only affects Codex's prompt, not how Claude Code processes results.

---

## 9. Error Handling

| Scenario | Behavior |
|----------|----------|
| `--language xyz` (unsupported) | `PreRunE` rejects with: `unsupported language "xyz"; supported: cpp` |
| `--language cpp` but `.md` file missing | Build-time error — embedded FS won't compile if file is missing |
| `--language` on resume call | Ignored silently — session language is authoritative |
| Mixed-language files | Skill's responsibility — don't pass `--language` |

---

## 10. Files Changed

| File | Change |
|------|--------|
| `cmd/xreview/cmd_review.go` | Add `--language` flag, validation in `PreRunE`, pass to `ReviewRequest` |
| `internal/reviewer/reviewer.go` | Add `Language string` to `ReviewRequest`, `ReviewResult`, `VerifyResult` |
| `internal/reviewer/single.go` | Set `sess.Language`, pass language to prompt builder, populate result |
| `internal/prompt/language_registry.go` | New file: `SupportedLanguages` map + `SupportedLanguageList()` |
| `internal/prompt/builder.go` | Add `Language` to input structs, append language section in Build methods |
| `internal/prompt/templates.go` | Add `LanguageSectionTemplate` constant |
| `internal/prompt/languages/embed.go` | New file: `//go:embed *.md` |
| `internal/prompt/languages/cpp.md` | New file: C++ review guidelines (content TBD) |
| `internal/session/types.go` | Add `Language string` to `Session` struct |
| `internal/formatter/xml.go` | Add `language` param to `FormatReviewResult()`, conditional attribute |
| `skills/review/SKILL.md` | Add supported languages table, update Step 2 examples |

---

## 11. What Does NOT Change

- `review.json` output schema — language is prompt context, not output structure
- `Finding` / `CodexFinding` structs — no language field on findings
- `Parser` — unchanged
- `CRITICAL_RULES` in `FirstRoundTemplate` — language rules are additive
- Session version (`CurrentSessionVersion = 2`) — `Language` is `omitempty`, backward compatible
- `session.Manager` interface — `Create()` signature unchanged; language set post-creation
- `config.json` — no project-level language setting
- `write-report` skill — report generation is language-agnostic

---

## 12. Future Considerations (Out of Scope)

- Additional languages: add `.md` file + map entry + skill table row
- Taste review (`review-taste`): separate command, separate design
- Custom guidelines (`--guideline <path>`): user-provided review rules, layered on top
- Auto-detection fallback in xreview binary: currently unnecessary since skill handles detection
