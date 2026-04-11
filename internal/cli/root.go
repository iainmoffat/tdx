package cli

import (
	"github.com/ipm/tdx/internal/cli/auth"
	"github.com/ipm/tdx/internal/cli/config"
	"github.com/spf13/cobra"
)

// NewRootCmd returns the top-level tdx command.
// version is injected at build time by cmd/tdx/main.go.
func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "tdx",
		Short:         "Manage TeamDynamix time entries from the terminal",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd(version))
	root.AddCommand(config.NewCmd())
	root.AddCommand(auth.NewCmd())
	return root
}
