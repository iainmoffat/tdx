// Package time wires the `tdx time` subtree. The package name intentionally
// shadows stdlib "time" in this directory only; callers outside this tree
// import it as internal/cli/time and reference its NewCmd function, so the
// shadow is harmless.
package time

import (
	"github.com/ipm/tdx/internal/cli/time/entry"
	"github.com/spf13/cobra"
)

// NewCmd returns the `tdx time` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "time",
		Short: "Read and manage TeamDynamix time entries",
	}
	cmd.AddCommand(entry.NewCmd())
	// week and timetype subtrees are added in Tasks 22 and 23.
	return cmd
}
