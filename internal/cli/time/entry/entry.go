package entry

import "github.com/spf13/cobra"

// NewCmd returns the `tdx time entry` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entry",
		Short: "Manage time entries",
	}
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newUpdateCmd())
	cmd.AddCommand(newDeleteCmd())
	return cmd
}
