package template

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/svc/tmplsvc"
)

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			store := tmplsvc.NewStore(paths)
			if err := store.Delete(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted template %q\n", args[0])
			return nil
		},
	}
}
