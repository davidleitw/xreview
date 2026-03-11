package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/version"
	"github.com/spf13/cobra"
)

func newPreflightCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preflight",
		Short: "Verify codex environment is ready",
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := runPreflightChecks()

			allPassed := true
			for _, c := range checks {
				if !c.Passed {
					allPassed = false
					break
				}
			}

			fmt.Println(formatter.FormatPreflightResult(checks, version.Version))

			if !allPassed {
				// Return error so exit code is non-zero, but the XML output
				// already contains all the detail Claude Code needs.
				return fmt.Errorf("preflight checks failed")
			}
			return nil
		},
	}
}

func runPreflightChecks() []formatter.Check {
	var checks []formatter.Check

	// Check 1: codex binary exists
	codexPath, err := exec.LookPath("codex")
	if err != nil {
		checks = append(checks, formatter.Check{
			Name:   "codex_installed",
			Passed: false,
			Detail: "codex CLI is not found in PATH. Please install it: npm install -g @openai/codex",
		})
		return checks
	}
	checks = append(checks, formatter.Check{
		Name:   "codex_installed",
		Passed: true,
		Detail: fmt.Sprintf("found at %s", codexPath),
	})

	// Check 2: codex version (basic responsiveness)
	out, err := exec.Command("codex", "--version").CombinedOutput()
	if err != nil {
		checks = append(checks, formatter.Check{
			Name:   "codex_responsive",
			Passed: false,
			Detail: fmt.Sprintf("codex --version failed: %v", err),
		})
		return checks
	}
	checks = append(checks, formatter.Check{
		Name:   "codex_responsive",
		Passed: true,
		Detail: strings.TrimSpace(string(out)),
	})

	return checks
}
