package week

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type listFlags struct {
	profile    string
	dirty      bool
	conflicted bool
	dateFilter string
	noRemote   bool
	json       bool
}

type weekDraftListItem struct {
	WeekStart  string               `json:"weekStart"`
	Name       string               `json:"name"`
	Profile    string               `json:"profile"`
	SyncState  string               `json:"syncState"`
	SyncDetail domain.DraftSyncState `json:"syncDetail"`
	TotalHours float64              `json:"totalHours"`
	PulledAt   string               `json:"pulledAt,omitempty"`
}

type weekDraftListResp struct {
	Schema string              `json:"schema"`
	Drafts []weekDraftListItem `json:"drafts"`
}

func newListCmd() *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local week drafts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name (defaults to active)")
	cmd.Flags().BoolVar(&f.dirty, "dirty", false, "show only dirty drafts")
	cmd.Flags().BoolVar(&f.conflicted, "conflicted", false, "show only conflicted drafts")
	cmd.Flags().StringVar(&f.dateFilter, "date", "", "filter by week-start date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&f.noRemote, "no-remote-check", false, "skip remote fingerprint probe")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runList(cmd *cobra.Command, f listFlags) error {
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

	list, err := drafts.Store().List(profileName)
	if err != nil {
		return err
	}

	items := make([]weekDraftListItem, 0, len(list))
	for _, d := range list {
		if f.dateFilter != "" && d.WeekStart.Format("2006-01-02") != f.dateFilter {
			continue
		}
		pulled, _ := drafts.PulledCellsByKey(profileName, d.WeekStart, d.Name)
		fingerprint := ""
		if !f.noRemote {
			fingerprint = drafts.ProbeRemoteFingerprint(cmd.Context(), profileName, d.WeekStart)
		}
		state := domain.ComputeSyncState(d, pulled, fingerprint)
		if f.dirty && state.Sync != domain.SyncDirty {
			continue
		}
		if f.conflicted && state.Sync != domain.SyncConflicted {
			continue
		}
		items = append(items, weekDraftListItem{
			WeekStart:  d.WeekStart.Format("2006-01-02"),
			Name:       d.Name,
			Profile:    d.Profile,
			SyncState:  string(state.Sync),
			SyncDetail: state,
			TotalHours: state.TotalHours,
			PulledAt:   formatRFC3339OrEmpty(d.Provenance.PulledAt),
		})
	}

	w := cmd.OutOrStdout()
	if f.json {
		return writeListJSON(w, items)
	}
	writeListText(w, items)
	return nil
}

func writeListJSON(w io.Writer, items []weekDraftListItem) error {
	return json.NewEncoder(w).Encode(weekDraftListResp{
		Schema: "tdx.v1.weekDraftList",
		Drafts: items,
	})
}

func writeListText(w io.Writer, items []weekDraftListItem) {
	if len(items) == 0 {
		fmt.Fprintln(w, "No drafts found.")
		return
	}
	fmt.Fprintf(w, "%-12s  %-10s  %-12s  %5s  %s\n",
		"WEEK", "NAME", "STATE", "HOURS", "PULLED")
	for _, it := range items {
		fmt.Fprintf(w, "%-12s  %-10s  %-12s  %5.1f  %s\n",
			it.WeekStart, it.Name, it.SyncState, it.TotalHours, it.PulledAt)
	}
}

// formatRFC3339OrEmpty returns the RFC3339 representation of t, or "" if zero.
func formatRFC3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
