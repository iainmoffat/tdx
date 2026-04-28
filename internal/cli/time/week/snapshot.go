package week

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type snapshotFlags struct {
	profile string
	keep    bool
	note    string
	json    bool
}

type weekDraftSnapshotResp struct {
	Schema   string                `json:"schema"`
	Snapshot draftsvc.SnapshotInfo `json:"snapshot"`
}

func newSnapshotCmd() *cobra.Command {
	var f snapshotFlags
	cmd := &cobra.Command{
		Use:   "snapshot <date>[/<name>]",
		Short: "Take a manual snapshot of a draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSnapshot(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().BoolVar(&f.keep, "keep", false, "pin the snapshot (exempt from auto-prune)")
	cmd.Flags().StringVar(&f.note, "note", "", "note to attach to the snapshot")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runSnapshot(cmd *cobra.Command, f snapshotFlags, ref string) error {
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

	info, err := drafts.Snapshots().Take(d, draftsvc.OpManual, f.note)
	if err != nil {
		return err
	}
	if f.keep {
		if err := drafts.Snapshots().Pin(profileName, weekStart, name, info.Sequence, f.note); err != nil {
			return err
		}
		info.Pinned = true
	}

	w := cmd.OutOrStdout()
	if f.json {
		return writeSnapshotJSON(w, info)
	}
	pin := ""
	if info.Pinned {
		pin = " (pinned)"
	}
	_, _ = fmt.Fprintf(w, "Snapshot %d taken for draft %s/%s%s.\n",
		info.Sequence, weekStart.Format("2006-01-02"), name, pin)
	return nil
}

func writeSnapshotJSON(w io.Writer, info draftsvc.SnapshotInfo) error {
	return json.NewEncoder(w).Encode(weekDraftSnapshotResp{
		Schema: "tdx.v1.weekDraftSnapshot", Snapshot: info,
	})
}
