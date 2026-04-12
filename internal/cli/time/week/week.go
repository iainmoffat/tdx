package week

import "github.com/spf13/cobra"

// NewCmd returns the `tdx time week` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "week",
		Short: "Inspect weekly reports and locked days",
	}
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newLockedCmd())
	return cmd
}
