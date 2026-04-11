package auth

import "github.com/spf13/cobra"

// NewCmd returns the `tdx auth` command tree with the production TTY token reader.
func NewCmd() *cobra.Command {
	return NewCmdWithTokenReader(ttyReader{})
}

// NewCmdWithTokenReader lets tests inject a fake token reader.
func NewCmdWithTokenReader(reader TokenReader) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage TeamDynamix authentication",
	}
	cmd.AddCommand(newProfileCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newLoginCmd(reader))
	return cmd
}
