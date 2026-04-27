package template

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
)

func newDeleteCmd() *cobra.Command {
	var profileFlag string

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			store := tmplsvc.NewStore(paths)
			if err := store.Delete(profile, args[0]); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "deleted template %q\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}
