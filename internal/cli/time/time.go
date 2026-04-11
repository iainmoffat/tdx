// Package time wires the `tdx time` subtree. The package name intentionally
// shadows stdlib "time" in this directory only; callers outside this tree
// import it as internal/cli/time and reference its NewCmd function, so the
// shadow is harmless.
package time

import (
	"github.com/ipm/tdx/internal/cli/time/entry"
	"github.com/ipm/tdx/internal/cli/time/week"
	"github.com/spf13/cobra"
)

// NewCmd returns the `tdx time` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "time",
		Short: "Read and manage TeamDynamix time entries",
	}
	cmd.AddCommand(entry.NewCmd())
	cmd.AddCommand(week.NewCmd())
	// timetype subtree is added in Task 23.
	return cmd
}
