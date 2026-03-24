# xreview XML Schema Reference

This document describes the XML output format of xreview CLI commands.
Claude Code skill uses this to parse xreview results.

## Envelope

All output is wrapped in:
<xreview-result status="success|error" action="..." session="..." round="N">

## Elements

### <finding>
Attributes: id, severity (high|medium|low), category, status (open|fixed|dismissed|reopened)
Children: <location>, <description>, <suggestion>, <code-snippet>, <verification>, <trigger>, <cascade-impact>, <fix-alternatives>

### <trigger>
Content: Concrete trigger condition (specific input/scenario). Child of `<finding>`.

### <cascade-impact>
Children: `<impact>` elements. Each describes a codebase location affected by fixing this finding.

### <fix-alternatives>
Children: `<alternative>` elements.
`<alternative>` attributes: label (A/B/C), effort (minimal|moderate|large), recommended (true|false)
Content: description of the fix approach.

### <location>
Attributes: file (path), line (number)

### <summary>
Attributes: total, open, fixed, dismissed

### <error>
Attributes: code (see error code table)
Content: human-readable error description with suggested action

### <checks> (preflight only)
Children: <check name="..." passed="true|false" detail="..." />

### <version> (version only)
Attributes: current, latest, outdated (true|false), update-command

## Error Codes

| Code | Meaning |
|------|---------|
| CODEX_NOT_FOUND | codex binary not in PATH |
| CODEX_NOT_AUTHENTICATED | codex not logged in |
| CODEX_UNRESPONSIVE | codex did not respond to test prompt |
| CODEX_TIMEOUT | codex exceeded timeout |
| CODEX_ERROR | codex exited with non-zero code |
| PARSE_FAILURE | could not parse codex output |
| SESSION_NOT_FOUND | session ID does not exist |
| NO_TARGETS | no files to review |
| INVALID_FLAGS | invalid flag combination |
| UPDATE_FAILED | self-update failed |
