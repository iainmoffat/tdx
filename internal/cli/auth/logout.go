package auth

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/spf13/cobra"
)

func newLogoutCmd() *cobra.Command {
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear the stored token for a profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			svc := authsvc.New(paths)

			profileName, err := svc.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			if err := svc.Logout(profileName); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "logged out of profile %q\n", profileName)
			return nil
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to the configured default)")
	return cmd
}
