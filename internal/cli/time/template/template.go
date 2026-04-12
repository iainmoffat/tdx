package template

import "github.com/spf13/cobra"

// NewCmd returns the `tdx time template` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage time entry templates",
	}
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newCloneCmd())
	cmd.AddCommand(newDeriveCmd())
	cmd.AddCommand(newApplyCmd())
	cmd.AddCommand(newCompareCmd())
	return cmd
}
