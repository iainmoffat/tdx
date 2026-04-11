package entry

import "github.com/spf13/cobra"

// NewCmd returns the `tdx time entry` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entry",
		Short: "List and inspect time entries",
	}
	cmd.AddCommand(newListCmd())
	// `show` is added in Task 21.
	return cmd
}
