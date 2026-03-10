# xreview Implementation Plan Index

> **For agentic workers:** Execute plans in order. Each plan is independent but builds on previous ones (dependency chain).

**Total Plans:** 10
**Estimated Scope:** ~50 files, ~3000 lines of Go code + tests

---

## Execution Order (dependency chain)

| # | Plan | Scope | Depends on |
|---|------|-------|------------|
| 1 | [Scaffold](2026-03-10-xreview-plan-01-scaffold.md) | go.mod, types, config, version, main.go, Makefile | — |
| 2 | [Session](2026-03-10-xreview-plan-02-session.md) | Session CRUD, findings state, comparison | Plan 1 |
| 3 | [Collector](2026-03-10-xreview-plan-03-collector.md) | File reading, git uncommitted detection | Plan 1 |
| 4 | [Prompt](2026-03-10-xreview-plan-04-prompt.md) | Prompt templates + builder | Plans 1, 3 |
| 5 | [Codex](2026-03-10-xreview-plan-05-codex.md) | Runner, schema, session ID extraction | Plan 1 |
| 6 | [Parser](2026-03-10-xreview-plan-06-parser.md) | JSON extraction + parsing + validation | Plan 1 |
| 7 | [Formatter](2026-03-10-xreview-plan-07-formatter.md) | XML output for all commands | Plan 1 |
| 8 | [CLI](2026-03-10-xreview-plan-08-cli.md) | All 6 cobra commands | Plans 1-7 |
| 9 | [Integration](2026-03-10-xreview-plan-09-integration.md) | Mock codex + integration tests | Plans 1-8 |
| 10 | [Skill](2026-03-10-xreview-plan-10-skill.md) | SKILL.md, reference.md, .gitignore | Plans 1-9 |

## Parallelism

Plans 2-7 can be executed **in parallel** (they only depend on Plan 1's types).
Plan 8 depends on all of 2-7.
Plans 9-10 are sequential after Plan 8.

```
Plan 1 (scaffold)
  |
  +---> Plan 2 (session)    --+
  +---> Plan 3 (collector)  --+
  +---> Plan 4 (prompt)     --+--> Plan 8 (CLI) --> Plan 9 (integration) --> Plan 10 (skill)
  +---> Plan 5 (codex)      --+
  +---> Plan 6 (parser)     --+
  +---> Plan 7 (formatter)  --+
```

## Design Doc Reference

All plans derive from: `docs/specs/2026-03-10-xreview-design.md`
