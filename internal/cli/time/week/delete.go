package week

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type deleteFlags struct {
	profile       string
	yes           bool
	keepSnapshots bool
}

func newDeleteCmd() *cobra.Command {
	var f deleteFlags
	cmd := &cobra.Command{
		Use:   "delete <date>[/<name>]",
		Short: "Delete a local draft (auto-snapshots first)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "confirm deletion")
	cmd.Flags().BoolVar(&f.keepSnapshots, "keep-snapshots", true, "keep snapshot history (default true)")
	return cmd
}

func runDelete(cmd *cobra.Command, f deleteFlags, ref string) error {
	if !f.yes {
		return fmt.Errorf("pass --yes to delete the draft (auto-snapshots first)")
	}

	weekStart, name, err := ParseDraftRef(ref)
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

	d, err := drafts.Store().Load(profileName, weekStart, name)
	if err != nil {
		return err
	}

	if _, err := drafts.Snapshots().Take(d, draftsvc.OpPreDelete, ""); err != nil {
		return fmt.Errorf("auto-snapshot pre-delete: %w", err)
	}
	if err := drafts.Store().Delete(profileName, weekStart, name); err != nil {
		return err
	}

	// Best-effort remove pulled-snapshot sibling.
	pulledPath := filepath.Join(paths.ProfileWeeksDir(profileName),
		weekStart.Format("2006-01-02"), name+".pulled.yaml")
	_ = os.Remove(pulledPath)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted draft %s/%s.\n",
		weekStart.Format("2006-01-02"), name)
	return nil
}
