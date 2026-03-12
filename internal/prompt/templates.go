package prompt

// FirstRoundTemplate is the prompt template for the initial review round.
const FirstRoundTemplate = `<CRITICAL_RULES>
1. PERFORM STATIC ANALYSIS ONLY. Do NOT execute or run the code.
2. Only report issues you can directly observe in the code.
   Do NOT speculate about issues in code you cannot see.
3. Every finding MUST reference a specific file and line number.
4. Focus on real bugs and security issues. Do NOT report trivial style preferences.
5. If you find no issues, set verdict to APPROVED with an empty findings array.
6. You MUST read additional files in the repository to understand the full context.
7. Review comprehensively: security, correctness, readability, maintainability,
   and extensibility. Do NOT limit your review to a single aspect.
8. Suggestions MUST be scoped and actionable within the current change.
   Do NOT suggest large-scale rewrites or architectural overhauls.
9. For each finding, check whether the same pattern exists in other functions or files.
   Report ALL instances, not just the first one you encounter.
10. Do NOT report style issues, naming conventions, or issues already marked with
    TODO, BUG, or FIXME comments in the code.
</CRITICAL_RULES>

You are a senior code reviewer. Analyze the code for bugs,
security vulnerabilities, logic errors, and significant quality issues.

Context from the developer: {{.Context}}

===== HOW TO GET THE CODE =====

{{.FetchMethod}}

Files involved:

{{.FileList}}

You MUST follow the instructions above to get the actual code.
Read additional files as needed for full context (callers, callees, type definitions, etc.).
Pay close attention to the developer context — it tells you what to focus on.

===== END =====

For each finding, you MUST also provide these fields:
- trigger: the concrete input, scenario, or call sequence that manifests the issue.
  Be specific (e.g. "user sends id=' OR '1'='1") not abstract (e.g. "malicious input").
- cascade_impact: other files/functions in the repository that would be affected if
  this code is changed. Trace the call chain. Use format "file:function() — description".
  You are encouraged to read additional files to identify these. Empty array [] if none.
- fix_alternatives: provide 2-3 fix approaches. Each has label (A/B/C), description,
  effort (minimal/moderate/large), and recommended (true for exactly one).
  Consider trade-offs: minimal fix vs. systemic improvement.`

// ResumeTemplate is the prompt template for follow-up review rounds.
const ResumeTemplate = `This is a follow-up review. You previously reviewed these files and
identified the findings listed below. The developer has made changes
and provided the following update:

Developer message: "{{.Message}}"

===== PREVIOUS FINDINGS =====

{{.PreviousFindings}}

===== HOW TO GET THE UPDATED CODE =====

{{.FetchMethod}}

Files involved:

{{.FileList}}

You MUST follow the instructions above to get the current code.
Read additional files as needed for full context.

===== END =====

For each previous finding, determine:
1. If claimed fixed: verify the fix is actually correct and complete.
2. If claimed false positive: evaluate whether the dismissal is reasonable.
3. If no update: re-evaluate against the current code.

Also check for regressions: did any of the fixes introduce NEW issues?
Only report a new finding if it is directly caused by or exposed by the fixes above.
Do NOT report pre-existing issues unrelated to the changes.

New findings (not in the previous list) should have status "open" and a new unique "id".

Respond with ONLY a JSON object (no markdown fences, no explanation before or after).
Use this exact schema:
{
  "verdict": "APPROVED or REVISE",
  "summary": "brief summary of your review",
  "findings": [
    {
      "id": "F-001",
      "severity": "high|medium|low",
      "category": "security|logic|performance|error-handling",
      "file": "path/to/file",
      "line": 42,
      "description": "what is wrong",
      "suggestion": "how to fix it",
      "code_snippet": "the relevant code",
      "status": "open|fixed|dismissed|reopened",
      "verification_note": "verification details or empty string",
      "trigger": "concrete trigger condition",
      "cascade_impact": ["file:func() — impact description"],
      "fix_alternatives": [
        {"label": "A", "description": "fix approach", "effort": "minimal|moderate|large", "recommended": true}
      ]
    }
  ]
}`
