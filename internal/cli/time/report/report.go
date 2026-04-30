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
