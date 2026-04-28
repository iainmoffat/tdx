package week

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type renameFlags struct {
	profile string
}

func newRenameCmd() *cobra.Command {
	var f renameFlags
	cmd := &cobra.Command{
		Use:   "rename <date>[/<oldName>] <newName>",
		Short: "Rename a draft (auto-snapshots first)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRename(cmd, f, args[0], args[1])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	return cmd
}

func runRename(cmd *cobra.Command, f renameFlags, srcRef, newName string) error {
	weekStart, oldName, err := ParseDraftRef(srcRef)
	if err != nil {
		return err
	}
	if newName == "" {
		return fmt.Errorf("newName cannot be empty")
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

	if err := drafts.Rename(profileName, weekStart, oldName, newName); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Renamed draft %s/%s -> %s/%s.\n",
		weekStart.Format("2006-01-02"), oldName, weekStart.Format("2006-01-02"), newName)
	return nil
}
