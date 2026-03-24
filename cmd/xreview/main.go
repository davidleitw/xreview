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

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xreview",
		Short: "Agent-Native Code Review Engine",
		Long:  "xreview orchestrates code review between Claude Code and Codex.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&flagWorkdir, "workdir", ".", "Working directory (default: current directory)")
	cmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Print debug information to stderr")
	cmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output raw JSON instead of XML")

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newSelfUpdateCmd())
	cmd.AddCommand(newPreflightCmd())
	cmd.AddCommand(newReviewCmd())
	cmd.AddCommand(newCleanCmd())

	return cmd
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
