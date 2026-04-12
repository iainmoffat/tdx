package timetype

import "github.com/spf13/cobra"

// NewCmd returns the `tdx time type` command tree. The package is named
// `timetype` because `type` is a Go keyword and cannot be used as a
// package identifier.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "type",
		Short: "List and look up TeamDynamix time types",
	}
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newForCmd())
	return cmd
}
