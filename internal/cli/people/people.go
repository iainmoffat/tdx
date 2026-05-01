package people

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

// peoplesvcAPI is the surface the people CLI needs. Defined here so tests
// can inject a stub without spinning up an HTTP client.
type peoplesvcAPI interface {
	SearchPools(ctx context.Context, profile string) ([]peoplesvc.ResourcePool, error)
}

// NewCmd returns the `tdx people` command tree wired against the
// production peoplesvc.Service.
func NewCmd() *cobra.Command {
	return newCmdWith(nil)
}

// newCmdWith lets tests inject a stub peoplesvc; nil means use the real one.
func newCmdWith(svc peoplesvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "people",
		Short: "Browse TD users, accounts, and resource pools",
	}
	cmd.AddCommand(newPoolsCmd(svc))
	return cmd
}
