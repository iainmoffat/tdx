package week

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type resetFlags struct {
	profile string
	yes     bool
}

func newResetCmd() *cobra.Command {
	var f resetFlags
	cmd := &cobra.Command{
		Use:   "reset <date>[/<name>]",
		Short: "Discard local edits and re-pull (auto-snapshots first)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !f.yes {
				return fmt.Errorf("pass --yes to discard local edits")
			}
			weekStart, name, err := ParseDraftRef(args[0])
			if err != nil {
				return err
			}

			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			tsvc := timesvc.New(paths)
			drafts := draftsvc.NewService(paths, tsvc)

			profileName, err := auth.ResolveProfile(f.profile)
			if err != nil {
				return err
			}

			if err := drafts.Reset(cmd.Context(), profileName, weekStart, name); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Reset draft %s/%s.\n",
				weekStart.Format("2006-01-02"), name)
			return nil
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "confirm discarding local edits")
	return cmd
}
