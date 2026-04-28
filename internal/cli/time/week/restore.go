package week

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type restoreFlags struct {
	profile  string
	snapshot int
	yes      bool
}

func newRestoreCmd() *cobra.Command {
	var f restoreFlags
	cmd := &cobra.Command{
		Use:   "restore <date>[/<name>] --snapshot N --yes",
		Short: "Restore a draft from a snapshot (auto-snapshots current first)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !f.yes {
				return fmt.Errorf("pass --yes to overwrite the current draft")
			}
			if f.snapshot <= 0 {
				return fmt.Errorf("--snapshot is required (use `tdx time week history` to find sequence numbers)")
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

			if err := drafts.RestoreSnapshot(profileName, weekStart, name, f.snapshot); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Restored draft %s/%s from snapshot %d.\n",
				weekStart.Format("2006-01-02"), name, f.snapshot)
			return nil
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().IntVar(&f.snapshot, "snapshot", 0, "snapshot sequence number to restore")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "confirm overwrite")
	return cmd
}
