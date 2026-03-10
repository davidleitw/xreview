package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPreflightCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "preflight",
		Short: "Verify codex environment is ready",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("preflight: not implemented")
		},
	}
}
