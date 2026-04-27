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

type historyFlags struct {
	profile string
	json    bool
	limit   int
}

type weekDraftSnapshotListResp struct {
	Schema    string                  `json:"schema"`
	Snapshots []draftsvc.SnapshotInfo `json:"snapshots"`
}

func newHistoryCmd() *cobra.Command {
	var f historyFlags
	cmd := &cobra.Command{
		Use:   "history <date>[/<name>]",
		Short: "List snapshots of a draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistory(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "show at most N most recent snapshots (0 = all)")
	return cmd
}

func runHistory(cmd *cobra.Command, f historyFlags, ref string) error {
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

	list, err := drafts.Snapshots().List(profileName, weekStart, name)
	if err != nil {
		return err
	}
	if f.limit > 0 && len(list) > f.limit {
		list = list[len(list)-f.limit:]
	}
	return renderHistory(cmd.OutOrStdout(), list, f.json)
}

func renderHistory(w io.Writer, list []draftsvc.SnapshotInfo, jsonOut bool) error {
	if jsonOut {
		return json.NewEncoder(w).Encode(weekDraftSnapshotListResp{
			Schema:    "tdx.v1.weekDraftSnapshotList",
			Snapshots: list,
		})
	}
	if len(list) == 0 {
		_, _ = fmt.Fprintln(w, "No snapshots.")
		return nil
	}
	_, _ = fmt.Fprintf(w, "%-4s  %-12s  %-20s  %-6s  %s\n", "SEQ", "OP", "TAKEN", "PINNED", "NOTE")
	for _, s := range list {
		pin := ""
		if s.Pinned {
			pin = "yes"
		}
		_, _ = fmt.Fprintf(w, "%-4d  %-12s  %-20s  %-6s  %s\n",
			s.Sequence, s.Op, s.Taken.UTC().Format("2006-01-02 15:04:05"), pin, s.Note)
	}
	return nil
}
