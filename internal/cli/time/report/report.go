// Package report provides the `tdx time report` command tree.
package report

import "github.com/spf13/cobra"

// NewCmd returns the `tdx time report` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate time-related reports",
	}
	cmd.AddCommand(newStatusCmd())
	return cmd
}

// newStatusCmd is the temporary stub for `tdx time report status`. Task 8
// replaces this with full flag wiring + selector validation.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Weekly time-status report (per user, per week)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Help()
			return nil
		},
	}
}
