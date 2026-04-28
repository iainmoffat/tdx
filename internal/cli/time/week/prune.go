package week

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type pruneFlags struct {
	profile   string
	olderThan string
	yes       bool
}

func newPruneCmd() *cobra.Command {
	var f pruneFlags
	cmd := &cobra.Command{
		Use:   "prune <date>[/<name>]",
		Short: "Drop unpinned snapshots older than --older-than (default: prune to retention cap)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !f.yes {
				return fmt.Errorf("pass --yes to actually delete snapshots")
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

			if f.olderThan != "" {
				d, err := parseShift(f.olderThan)
				if err != nil {
					return err
				}
				if d < 0 {
					return fmt.Errorf("--older-than cannot be negative")
				}
				n, err := drafts.Snapshots().PruneOlderThan(profileName, weekStart, name, d)
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d snapshot(s) older than %s.\n", n, f.olderThan)
				return nil
			}

			n, err := drafts.Snapshots().PruneToRetention(profileName, weekStart, name)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d snapshot(s) beyond retention cap.\n", n)
			return nil
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().StringVar(&f.olderThan, "older-than", "", "prune snapshots older than this (e.g. 30d, 7d)")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "confirm deletion")
	return cmd
}
