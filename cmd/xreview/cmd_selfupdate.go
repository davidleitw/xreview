package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSelfUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "self-update",
		Short: "Update xreview to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("self-update: not implemented")
		},
	}
}
