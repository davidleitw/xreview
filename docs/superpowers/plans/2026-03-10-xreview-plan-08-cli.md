# Plan 8: CLI Commands — All 6 Cobra Subcommands

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire up all 6 CLI commands (version, self-update, preflight, review, report, clean) using cobra, connecting the internal packages into a working CLI.

**Architecture:** Each command in its own file under `cmd/xreview/`. Commands orchestrate internal packages — they don't contain business logic. Global flags (--workdir, --verbose, --json) handled by root command.

**Tech Stack:** github.com/spf13/cobra, all internal/* packages

**Depends on:** Plans 1-7 (all internal packages)

---

## Chunk 1: Simple Commands — version, self-update, clean

### Task 8.1: Version Command

**Files:**
- Create: `cmd/xreview/cmd_version.go`
- Modify: `cmd/xreview/main.go` (register command)
- Modify: `internal/version/version.go` (add latest version check)

- [ ] **Step 1: Add latest version check to version package**

Create or update `internal/version/version.go` to add `CheckLatest()`:

```go
package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	Version = "dev"
)

// LatestInfo holds the result of a version check.
type LatestInfo struct {
	Current  string
	Latest   string
	Outdated bool
}

// CheckLatest queries GitHub API for the latest release version.
func CheckLatest() (*LatestInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/davidleitw/xreview/releases/latest")
	if err != nil {
		return nil, fmt.Errorf("version check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// No releases yet — current is latest
		return &LatestInfo{Current: Version, Latest: Version, Outdated: false}, nil
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	return &LatestInfo{
		Current:  Version,
		Latest:   latest,
		Outdated: latest != Version && Version != "dev",
	}, nil
}
```

- [ ] **Step 2: Write cmd_version.go**

Create `cmd/xreview/cmd_version.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version and check for updates",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := version.CheckLatest()
			if err != nil {
				xml, _ := formatter.RenderError("version", "VERSION_CHECK_FAILED",
					fmt.Sprintf("Could not check for updates: %v. Current version: %s", err, version.Version))
				fmt.Fprint(os.Stdout, xml)
				return nil
			}

			xml, err := formatter.RenderVersion(info.Current, info.Latest, info.Outdated)
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, xml)
			return nil
		},
	}
}
```

- [ ] **Step 3: Register in main.go — update rootCmd to add all commands**

Update `cmd/xreview/main.go` to register commands:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagWorkdir string
	flagVerbose bool
	flagJSON    bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "xreview",
		Short:        "Agent-Native Code Review Engine",
		Long:         "xreview orchestrates code review between Claude Code and Codex.",
		SilenceUsage: true,
	}

	rootCmd.PersistentFlags().StringVar(&flagWorkdir, "workdir", "", "Override working directory (default: current directory)")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Print debug information to stderr")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output raw JSON instead of XML")

	rootCmd.AddCommand(newVersionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// getWorkdir returns the effective working directory.
func getWorkdir() string {
	if flagWorkdir != "" {
		return flagWorkdir
	}
	dir, _ := os.Getwd()
	return dir
}
```

- [ ] **Step 4: Verify build and run**

```bash
cd /home/davidleitw/xreview && make build && ./bin/xreview version
```

Expected: XML output with version info

- [ ] **Step 5: Commit**

```bash
git add cmd/xreview/ internal/version/
git commit -m "feat: add version command with GitHub latest check"
```

---

### Task 8.2: Clean Command

**Files:**
- Create: `cmd/xreview/cmd_clean.go`
- Modify: `cmd/xreview/main.go` (register)

- [ ] **Step 1: Write cmd_clean.go**

Create `cmd/xreview/cmd_clean.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/session"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Delete session data",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				xml, _ := formatter.RenderError("clean", "INVALID_FLAGS", "Missing required flag --session.")
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			}

			mgr := session.NewManager(getWorkdir())

			if err := mgr.Delete(sessionID); err != nil {
				xml, _ := formatter.RenderError("clean", "SESSION_NOT_FOUND",
					fmt.Sprintf("Session '%s' not found. %v", sessionID, err))
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			}

			xml, _ := formatter.RenderClean(sessionID)
			fmt.Fprint(os.Stdout, xml)
			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to delete (required)")
	return cmd
}
```

- [ ] **Step 2: Register in main.go**

Add `rootCmd.AddCommand(newCleanCmd())` in main.go.

- [ ] **Step 3: Verify build**

```bash
cd /home/davidleitw/xreview && make build && ./bin/xreview clean --help
```

- [ ] **Step 4: Commit**

```bash
git add cmd/xreview/cmd_clean.go cmd/xreview/main.go
git commit -m "feat: add clean command"
```

---

### Task 8.3: Self-Update Command

**Files:**
- Create: `cmd/xreview/cmd_selfupdate.go`

- [ ] **Step 1: Write cmd_selfupdate.go**

Create `cmd/xreview/cmd_selfupdate.go`:

```go
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/version"
	"github.com/spf13/cobra"
)

func newSelfUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "self-update",
		Short: "Update to latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			oldVersion := version.Version

			// Check if Go is available
			goPath, err := exec.LookPath("go")
			if err != nil {
				xml, _ := formatter.RenderError("self-update", "UPDATE_FAILED",
					"Go is not installed. Please install Go or download the binary from https://github.com/davidleitw/xreview/releases")
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			}

			// Run go install
			installCmd := exec.Command(goPath, "install", "github.com/davidleitw/xreview@latest")
			installCmd.Stderr = os.Stderr
			if err := installCmd.Run(); err != nil {
				xml, _ := formatter.RenderError("self-update", "UPDATE_FAILED",
					fmt.Sprintf("Failed to update: %v. Try manually: go install github.com/davidleitw/xreview@latest", err))
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			}

			// Check new version
			info, err := version.CheckLatest()
			newVersion := "unknown"
			if err == nil {
				newVersion = info.Latest
			}

			alreadyLatest := oldVersion == newVersion
			xml, _ := formatter.RenderSelfUpdate(oldVersion, newVersion, alreadyLatest)
			fmt.Fprint(os.Stdout, xml)
			return nil
		},
	}
}
```

- [ ] **Step 2: Register in main.go**

Add `rootCmd.AddCommand(newSelfUpdateCmd())`

- [ ] **Step 3: Commit**

```bash
git add cmd/xreview/cmd_selfupdate.go cmd/xreview/main.go
git commit -m "feat: add self-update command"
```

---

## Chunk 2: Preflight Command

### Task 8.4: Preflight Command

**Files:**
- Create: `cmd/xreview/cmd_preflight.go`

- [ ] **Step 1: Write cmd_preflight.go**

Create `cmd/xreview/cmd_preflight.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/spf13/cobra"
)

func newPreflightCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preflight",
		Short: "Verify codex environment is ready",
		RunE: func(cmd *cobra.Command, args []string) error {
			var checks []formatter.PreflightCheck

			// Check 1: codex binary exists
			codexPath, err := exec.LookPath("codex")
			if err != nil {
				checks = append(checks, formatter.PreflightCheck{
					Name: "codex_installed", Passed: false,
				})
				xml, _ := formatter.RenderPreflightError("CODEX_NOT_FOUND",
					"codex CLI is not found in PATH. Please install it. If using npm: npm install -g @openai/codex",
					checks)
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			}
			checks = append(checks, formatter.PreflightCheck{
				Name: "codex_installed", Passed: true,
				Detail: fmt.Sprintf("codex found at %s", codexPath),
			})

			// Check 2: codex is authenticated
			authCmd := exec.Command("codex", "auth", "status")
			if authOut, err := authCmd.CombinedOutput(); err != nil {
				checks = append(checks, formatter.PreflightCheck{
					Name: "codex_authenticated", Passed: false,
				})
				xml, _ := formatter.RenderPreflightError("CODEX_NOT_AUTHENTICATED",
					"codex is installed but not logged in. Please ask the user to run: codex login",
					checks)
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			} else {
				checks = append(checks, formatter.PreflightCheck{
					Name: "codex_authenticated", Passed: true,
					Detail: truncate(string(authOut), 100),
				})
			}

			// Check 3: codex can respond
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			start := time.Now()
			testCmd := exec.CommandContext(ctx, "codex", "exec", "--skip-git-repo-check", "respond with OK")
			if _, err := testCmd.CombinedOutput(); err != nil {
				checks = append(checks, formatter.PreflightCheck{
					Name: "codex_responsive", Passed: false,
				})
				xml, _ := formatter.RenderPreflightError("CODEX_UNRESPONSIVE",
					"codex is installed and authenticated but not responding. This could be a network issue or service outage.",
					checks)
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			}
			elapsed := time.Since(start)
			checks = append(checks, formatter.PreflightCheck{
				Name: "codex_responsive", Passed: true,
				Detail: fmt.Sprintf("codex responded in %.1fs", elapsed.Seconds()),
			})

			xml, _ := formatter.RenderPreflight(checks)
			fmt.Fprint(os.Stdout, xml)
			return nil
		},
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
```

- [ ] **Step 2: Register in main.go**

Add `rootCmd.AddCommand(newPreflightCmd())`

- [ ] **Step 3: Commit**

```bash
git add cmd/xreview/cmd_preflight.go cmd/xreview/main.go
git commit -m "feat: add preflight command with codex environment checks"
```

---

## Chunk 3: Review Command (the core)

### Task 8.5: Review Command

**Files:**
- Create: `cmd/xreview/cmd_review.go`

- [ ] **Step 1: Write cmd_review.go**

Create `cmd/xreview/cmd_review.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidleitw/xreview/internal/codex"
	"github.com/davidleitw/xreview/internal/collector"
	"github.com/davidleitw/xreview/internal/config"
	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/parser"
	"github.com/davidleitw/xreview/internal/prompt"
	"github.com/davidleitw/xreview/internal/schema"
	"github.com/davidleitw/xreview/internal/session"
	"github.com/spf13/cobra"
)

func newReviewCmd() *cobra.Command {
	var (
		files          string
		gitUncommitted bool
		contextMsg     string
		sessionID      string
		message        string
		fullRescan     bool
		timeout        int
	)

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Run a new review or continue an existing session",
		RunE: func(cmd *cobra.Command, args []string) error {
			workdir := getWorkdir()

			// Load config
			cfg, err := config.Load(workdir)
			if err != nil {
				if flagVerbose {
					fmt.Fprintf(os.Stderr, "config load warning: %v\n", err)
				}
				cfg = &config.Config{
					CodexModel:     config.DefaultCodexModel,
					DefaultTimeout: config.DefaultTimeout,
				}
			}
			if timeout == 0 {
				timeout = cfg.DefaultTimeout
			}

			// Validate flags
			if err := validateReviewFlags(files, gitUncommitted, sessionID, message, fullRescan); err != nil {
				xml, _ := formatter.RenderError("review", "INVALID_FLAGS", err.Error())
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			}

			mgr := session.NewManager(workdir)

			if sessionID != "" {
				return runResumeReview(mgr, cfg, sessionID, message, fullRescan, timeout, workdir)
			}
			return runNewReview(mgr, cfg, files, gitUncommitted, contextMsg, timeout, workdir)
		},
	}

	cmd.Flags().StringVar(&files, "files", "", "Comma-separated file paths to review")
	cmd.Flags().BoolVar(&gitUncommitted, "git-uncommitted", false, "Review all uncommitted changes")
	cmd.Flags().StringVar(&contextMsg, "context", "", "Description of what was implemented")
	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to continue")
	cmd.Flags().StringVar(&message, "message", "", "Description of fixes/dismissals")
	cmd.Flags().BoolVar(&fullRescan, "full-rescan", false, "Fresh codex session, compare against previous")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Codex timeout in seconds (default from config)")

	return cmd
}

func validateReviewFlags(files string, gitUncommitted bool, sessionID, message string, fullRescan bool) error {
	if files != "" && gitUncommitted {
		return fmt.Errorf("--files and --git-uncommitted are mutually exclusive")
	}
	if sessionID != "" && (files != "" || gitUncommitted) {
		return fmt.Errorf("--files/--git-uncommitted cannot be used with --session")
	}
	if sessionID == "" && (message != "" || fullRescan) {
		return fmt.Errorf("--message and --full-rescan require --session")
	}
	if sessionID == "" && files == "" && !gitUncommitted {
		return fmt.Errorf("specify --files or --git-uncommitted for a new review, or --session to continue")
	}
	return nil
}

func runNewReview(mgr *session.Manager, cfg *config.Config, filesFlag string, gitUncommitted bool, contextMsg string, timeout int, workdir string) error {
	// Collect files
	var filePaths []string
	var diff string
	var targetMode string

	if gitUncommitted {
		var err error
		filePaths, err = collector.GitUncommittedFiles(workdir)
		if err != nil {
			xml, _ := formatter.RenderError("review", "NOT_GIT_REPO", err.Error())
			fmt.Fprint(os.Stdout, xml)
			os.Exit(1)
		}
		if len(filePaths) == 0 {
			xml, _ := formatter.RenderError("review", "NO_TARGETS",
				"No uncommitted changes found. Either specify files with --files, or make some changes.")
			fmt.Fprint(os.Stdout, xml)
			os.Exit(1)
		}
		diff, _ = collector.GitDiff(workdir)
		targetMode = "git-uncommitted"
	} else {
		filePaths = strings.Split(filesFlag, ",")
		for i := range filePaths {
			filePaths[i] = strings.TrimSpace(filePaths[i])
			if !filepath.IsAbs(filePaths[i]) {
				filePaths[i] = filepath.Join(workdir, filePaths[i])
			}
		}
		// Verify files exist
		for _, p := range filePaths {
			if _, err := os.Stat(p); os.IsNotExist(err) {
				xml, _ := formatter.RenderError("review", "FILE_NOT_FOUND",
					fmt.Sprintf("File not found: %s. Please check the file path.", p))
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			}
		}
		targetMode = "files"
	}

	// Collect file contents
	collected, err := collector.CollectFiles(filePaths)
	if err != nil {
		xml, _ := formatter.RenderError("review", "FILE_NOT_FOUND", err.Error())
		fmt.Fprint(os.Stdout, xml)
		os.Exit(1)
	}

	// Create session
	relPaths := make([]string, len(filePaths))
	for i, p := range filePaths {
		rel, err := filepath.Rel(workdir, p)
		if err != nil {
			rel = p
		}
		relPaths[i] = rel
	}
	sess, err := mgr.Create(relPaths, targetMode, contextMsg, timeout)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Write schema file
	schemaPath := filepath.Join(mgr.SessionDir(sess.SessionID), "schema.json")
	if err := schema.WriteSchema(schemaPath); err != nil {
		return fmt.Errorf("write schema: %w", err)
	}

	// Build prompt
	fileList := collector.FormatFileList(collected)
	if diff == "" {
		diff = "(no diff available — files specified explicitly)"
	}
	promptStr, err := prompt.BuildFirstRound(contextMsg, fileList, diff)
	if err != nil {
		return fmt.Errorf("build prompt: %w", err)
	}

	// Run codex
	runner := codex.NewRunner(cfg.CodexModel, schemaPath)
	start := time.Now()
	result, err := runner.Exec(context.Background(), promptStr, "", timeout)
	duration := time.Since(start)

	if result != nil {
		mgr.SaveRawOutput(sess.SessionID, 1, result.Stdout, result.Stderr)
	}

	if err != nil {
		errCode := "CODEX_ERROR"
		if strings.Contains(err.Error(), "timed out") {
			errCode = "CODEX_TIMEOUT"
		}
		xml, _ := formatter.RenderError("review", errCode, err.Error())
		fmt.Fprint(os.Stdout, xml)
		os.Exit(1)
	}

	// Parse codex output
	resp, err := parser.Parse(result.Stdout)
	if err != nil {
		xml, _ := formatter.RenderError("review", "PARSE_FAILURE",
			fmt.Sprintf("Could not parse codex output. Raw output saved to .xreview/sessions/%s/raw/. %v", sess.SessionID, err))
		fmt.Fprint(os.Stdout, xml)
		os.Exit(1)
	}

	// Process findings
	findings := session.AssignFindingIDs(resp.Findings, 1)
	summary := session.ComputeSummary(findings)

	// Update session
	sess.Status = session.StatusInReview
	sess.CurrentRound = 1
	sess.CodexSessionID = result.SessionID
	sess.CodexModel = cfg.CodexModel
	mgr.Save(sess)

	// Save findings
	mgr.SaveFindings(sess.SessionID, &session.FindingsFile{
		LastUpdatedRound: 1,
		Findings:         findings,
		Summary:          summary,
	})

	// Save round
	mgr.SaveRound(sess.SessionID, &session.Round{
		RoundNum:       1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Action:         "review",
		CodexSessionID: result.SessionID,
		CodexResumed:   false,
		Targets:        relPaths,
		FindingsAfter:  summary,
		RawStdoutPath:  "raw/round-001-codex-stdout.txt",
		RawStderrPath:  "raw/round-001-codex-stderr.txt",
		DurationMs:     duration.Milliseconds(),
	})

	// Output XML
	xml, _ := formatter.RenderReview(sess.SessionID, 1, findings, summary, false)
	fmt.Fprint(os.Stdout, xml)
	return nil
}

func runResumeReview(mgr *session.Manager, cfg *config.Config, sessionID, message string, fullRescan bool, timeout int, workdir string) error {
	// Load session
	sess, err := mgr.Load(sessionID)
	if err != nil {
		ids, _ := mgr.List()
		xml, _ := formatter.RenderError("review", "SESSION_NOT_FOUND",
			fmt.Sprintf("Session '%s' not found. Available sessions: %s", sessionID, strings.Join(ids, ", ")))
		fmt.Fprint(os.Stdout, xml)
		os.Exit(1)
	}

	// Load previous findings
	ff, err := mgr.LoadFindings(sessionID)
	if err != nil {
		return fmt.Errorf("load findings: %w", err)
	}

	// Collect current file contents
	absPaths := make([]string, len(sess.Targets))
	for i, t := range sess.Targets {
		absPaths[i] = filepath.Join(workdir, t)
	}
	collected, err := collector.CollectFiles(absPaths)
	if err != nil {
		xml, _ := formatter.RenderError("review", "FILE_NOT_FOUND", err.Error())
		fmt.Fprint(os.Stdout, xml)
		os.Exit(1)
	}

	// Schema path
	schemaPath := filepath.Join(mgr.SessionDir(sessionID), "schema.json")
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		schema.WriteSchema(schemaPath)
	}

	nextRound := sess.CurrentRound + 1
	var promptStr string
	resumeSessionID := sess.CodexSessionID

	if fullRescan {
		fileList := collector.FormatFileList(collected)
		diff, _ := collector.GitDiff(workdir)
		if diff == "" {
			diff = "(no diff available)"
		}
		promptStr, err = prompt.BuildFullRescan(sess.Context, ff.Findings, fileList, diff)
		resumeSessionID = "" // new codex session
	} else {
		var filesContent strings.Builder
		for _, f := range collected {
			fmt.Fprintf(&filesContent, "--- %s ---\n%s\n", f.Path, f.ContentWithLineNumbers)
		}
		promptStr, err = prompt.BuildResume(message, ff.Findings, filesContent.String())
	}
	if err != nil {
		return fmt.Errorf("build prompt: %w", err)
	}

	// Run codex
	runner := codex.NewRunner(cfg.CodexModel, schemaPath)
	start := time.Now()
	result, err := runner.Exec(context.Background(), promptStr, resumeSessionID, timeout)
	duration := time.Since(start)

	if result != nil {
		mgr.SaveRawOutput(sessionID, nextRound, result.Stdout, result.Stderr)
	}

	if err != nil {
		// If resume failed, try without resume
		if resumeSessionID != "" && !fullRescan {
			if flagVerbose {
				fmt.Fprintf(os.Stderr, "resume failed, retrying without resume: %v\n", err)
			}
			result, err = runner.Exec(context.Background(), promptStr, "", timeout)
			if result != nil {
				mgr.SaveRawOutput(sessionID, nextRound, result.Stdout, result.Stderr)
			}
		}
		if err != nil {
			errCode := "CODEX_ERROR"
			if strings.Contains(err.Error(), "timed out") {
				errCode = "CODEX_TIMEOUT"
			}
			xml, _ := formatter.RenderError("review", errCode, err.Error())
			fmt.Fprint(os.Stdout, xml)
			os.Exit(1)
		}
	}

	// Parse
	resp, err := parser.Parse(result.Stdout)
	if err != nil {
		xml, _ := formatter.RenderError("review", "PARSE_FAILURE", err.Error())
		fmt.Fprint(os.Stdout, xml)
		os.Exit(1)
	}

	// Process findings
	var findings []session.Finding
	var summary session.FindingSummary
	var resolved []session.ResolvedFinding

	summaryBefore := ff.Summary

	if fullRescan {
		compResult := session.CompareFindings(ff.Findings, resp.Findings, nextRound)
		findings = compResult.Findings
		resolved = compResult.Resolved
		summary = session.ComputeSummary(findings)
	} else {
		findings = session.MergeFindings(ff.Findings, resp.Findings, nextRound)
		summary = session.ComputeSummary(findings)
	}

	// Update session
	sess.Status = session.StatusVerifying
	sess.CurrentRound = nextRound
	if result.SessionID != "" {
		sess.CodexSessionID = result.SessionID
	}
	mgr.Save(sess)

	// Save findings
	mgr.SaveFindings(sessionID, &session.FindingsFile{
		LastUpdatedRound: nextRound,
		Findings:         findings,
		Summary:          summary,
	})

	// Save round
	codexResumed := resumeSessionID != "" && !fullRescan
	action := "verify"
	if fullRescan {
		action = "review"
	}
	mgr.SaveRound(sessionID, &session.Round{
		RoundNum:       nextRound,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Action:         action,
		CodexSessionID: result.SessionID,
		CodexResumed:   codexResumed,
		FullRescan:     fullRescan,
		UserMessage:    message,
		Targets:        sess.Targets,
		FindingsBefore: summaryBefore,
		FindingsAfter:  summary,
		RawStdoutPath:  fmt.Sprintf("raw/round-%03d-codex-stdout.txt", nextRound),
		RawStderrPath:  fmt.Sprintf("raw/round-%03d-codex-stderr.txt", nextRound),
		DurationMs:     duration.Milliseconds(),
	})

	// Output XML
	if fullRescan {
		xml, _ := formatter.RenderFullRescan(sessionID, nextRound, findings, resolved, summary)
		fmt.Fprint(os.Stdout, xml)
	} else {
		xml, _ := formatter.RenderReview(sessionID, nextRound, findings, summary, false)
		fmt.Fprint(os.Stdout, xml)
	}
	return nil
}
```

- [ ] **Step 2: Register in main.go**

Add `rootCmd.AddCommand(newReviewCmd())`

- [ ] **Step 3: Verify build**

```bash
cd /home/davidleitw/xreview && make build && ./bin/xreview review --help
```

Expected: Help text with all review flags

- [ ] **Step 4: Commit**

```bash
git add cmd/xreview/cmd_review.go cmd/xreview/main.go
git commit -m "feat: add review command with new, resume, and full-rescan modes"
```

---

## Chunk 4: Report Command

### Task 8.6: Report Command

**Files:**
- Create: `cmd/xreview/cmd_report.go`

- [ ] **Step 1: Write cmd_report.go**

Create `cmd/xreview/cmd_report.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/session"
	"github.com/spf13/cobra"
)

func newReportCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a review report",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				xml, _ := formatter.RenderError("report", "INVALID_FLAGS", "Missing required flag --session.")
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			}

			mgr := session.NewManager(getWorkdir())

			sess, err := mgr.Load(sessionID)
			if err != nil {
				ids, _ := mgr.List()
				xml, _ := formatter.RenderError("report", "SESSION_NOT_FOUND",
					fmt.Sprintf("Session '%s' not found. Available sessions: %s", sessionID, strings.Join(ids, ", ")))
				fmt.Fprint(os.Stdout, xml)
				os.Exit(1)
			}

			ff, err := mgr.LoadFindings(sessionID)
			if err != nil {
				return err
			}

			// Generate report markdown
			report := generateReport(sess, ff)
			reportPath := filepath.Join(mgr.SessionDir(sessionID), "report.md")
			if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
				return err
			}

			// Update session status
			sess.Status = session.StatusCompleted
			mgr.Save(sess)

			// Output XML
			relPath := fmt.Sprintf(".xreview/sessions/%s/report.md", sessionID)
			xml, _ := formatter.RenderReport(sessionID, relPath, sess.CurrentRound, ff.Summary)
			fmt.Fprint(os.Stdout, xml)
			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID (required)")
	return cmd
}

func generateReport(sess *session.Session, ff *session.FindingsFile) string {
	var b strings.Builder

	b.WriteString("# Code Review Report\n\n")
	fmt.Fprintf(&b, "**Session:** %s\n", sess.SessionID)
	fmt.Fprintf(&b, "**Date:** %s\n", sess.CreatedAt[:10])
	fmt.Fprintf(&b, "**Rounds:** %d\n", sess.CurrentRound)
	fmt.Fprintf(&b, "**Files Reviewed:** %s\n\n", strings.Join(sess.Targets, ", "))

	b.WriteString("## Summary\n\n")
	b.WriteString("| Status | Count |\n")
	b.WriteString("|--------|-------|\n")
	fmt.Fprintf(&b, "| Fixed | %d |\n", ff.Summary.Fixed)
	fmt.Fprintf(&b, "| Dismissed | %d |\n", ff.Summary.Dismissed)
	fmt.Fprintf(&b, "| Open | %d |\n", ff.Summary.Open)
	fmt.Fprintf(&b, "| **Total** | **%d** |\n\n", ff.Summary.Total)

	b.WriteString("## Findings\n\n")
	for _, f := range ff.Findings {
		fmt.Fprintf(&b, "### [%s] %s - %s | %s:%d | %s\n\n",
			f.ID,
			strings.ToUpper(f.Severity),
			f.Category,
			f.File,
			f.Line,
			strings.ToUpper(f.Status),
		)
		fmt.Fprintf(&b, "%s\n\n", f.Description)

		for _, h := range f.History {
			fmt.Fprintf(&b, "- **Round %d:** %s", h.Round, strings.ToUpper(h.Status))
			if h.Note != "" {
				fmt.Fprintf(&b, " — %s", h.Note)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}
```

- [ ] **Step 2: Register in main.go**

Add `rootCmd.AddCommand(newReportCmd())`

- [ ] **Step 3: Verify build**

```bash
cd /home/davidleitw/xreview && make build && ./bin/xreview report --help
```

- [ ] **Step 4: Commit**

```bash
git add cmd/xreview/cmd_report.go cmd/xreview/main.go
git commit -m "feat: add report command with markdown generation"
```

---

### Task 8.7: Final main.go — Register All Commands

**Files:**
- Modify: `cmd/xreview/main.go`

- [ ] **Step 1: Ensure all commands registered**

Verify `main.go` has:

```go
rootCmd.AddCommand(
    newVersionCmd(),
    newSelfUpdateCmd(),
    newPreflightCmd(),
    newReviewCmd(),
    newReportCmd(),
    newCleanCmd(),
)
```

- [ ] **Step 2: Full build + help test**

```bash
cd /home/davidleitw/xreview && make build && ./bin/xreview --help
```

Expected: All 6 commands listed in help

- [ ] **Step 3: Commit**

```bash
git add cmd/xreview/main.go
git commit -m "feat: register all 6 CLI commands in root"
```
