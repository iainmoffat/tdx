package week

import "github.com/spf13/cobra"

// NewCmd returns the `tdx time week` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "week",
		Short: "Inspect weekly reports and manage week drafts",
	}
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newLockedCmd())
	cmd.AddCommand(newPullCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newDiffCmd())
	cmd.AddCommand(newPreviewCmd())
	cmd.AddCommand(newPushCmd())
	return cmd
}
