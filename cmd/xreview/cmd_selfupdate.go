package main

import (
	"fmt"

	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/davidleitw/xreview/internal/updater"
	"github.com/spf13/cobra"
)

func newSelfUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "self-update",
		Short: "Update xreview to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			newVersion, err := updater.SelfUpdate()
			if err != nil {
				fmt.Println(formatter.FormatError("self-update", formatter.ErrUpdateFailed, err.Error()))
				return fmt.Errorf("self-update failed: %w", err)
			}

			fmt.Println(formatter.FormatSelfUpdateResult(newVersion))
			return nil
		},
	}
}
