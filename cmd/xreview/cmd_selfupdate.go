package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/davidleitw/xreview/internal/formatter"
	"github.com/spf13/cobra"
)

func newSelfUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "self-update",
		Short: "Update xreview to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := exec.Command("go", "install", "github.com/davidleitw/xreview@latest").CombinedOutput()
			if err != nil {
				msg := fmt.Sprintf("go install failed: %v\n%s", err, strings.TrimSpace(string(out)))
				fmt.Println(formatter.FormatError("self-update", formatter.ErrUpdateFailed, msg))
				return fmt.Errorf("self-update failed: %w", err)
			}

			fmt.Println("xreview updated successfully.")
			return nil
		},
	}
}
