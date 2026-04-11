package auth

import "github.com/spf13/cobra"

// NewCmd returns the `tdx auth` command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage TeamDynamix authentication",
	}
	cmd.AddCommand(newProfileCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newLogoutCmd())
	// login is added in the next task.
	return cmd
}
