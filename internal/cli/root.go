package cli

import (
	"github.com/iainmoffat/tdx/internal/cli/auth"
	"github.com/iainmoffat/tdx/internal/cli/config"
	mcpcli "github.com/iainmoffat/tdx/internal/cli/mcp"
	timecli "github.com/iainmoffat/tdx/internal/cli/time"
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
	root.AddCommand(timecli.NewCmd())
	root.AddCommand(mcpcli.NewCmd())
	root.AddCommand(newCompletionCmd())
	return root
}
