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
	// login and logout are added in later tasks.
	return cmd
}
