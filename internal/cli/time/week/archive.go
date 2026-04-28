package week

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

func newArchiveCmd() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "archive <date>[/<name>]",
		Short: "Hide a draft from default `list` output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArchive(cmd, profile, args[0], true)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func newUnarchiveCmd() *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "unarchive <date>[/<name>]",
		Short: "Show a previously archived draft in default `list` output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArchive(cmd, profile, args[0], false)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "profile name")
	return cmd
}

func runArchive(cmd *cobra.Command, profileFlag, ref string, archive bool) error {
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

	profileName, err := auth.ResolveProfile(profileFlag)
	if err != nil {
		return err
	}

	if err := drafts.SetArchived(profileName, weekStart, name, archive); err != nil {
		return err
	}
	verb := "Archived"
	if !archive {
		verb = "Unarchived"
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s draft %s/%s.\n",
		verb, weekStart.Format("2006-01-02"), name)
	return nil
}
