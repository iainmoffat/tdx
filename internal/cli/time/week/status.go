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

type statusFlags struct {
	profile  string
	json     bool
	noRemote bool
}

type weekDraftStatusResp struct {
	Schema            string                `json:"schema"`
	Profile           string                `json:"profile"`
	WeekStart         string                `json:"weekStart"`
	Name              string                `json:"name"`
	SyncState         string                `json:"syncState"`
	SyncDetail        domain.DraftSyncState `json:"syncDetail"`
	TotalHours        float64               `json:"totalHours"`
	PulledAt          string                `json:"pulledAt,omitempty"`
	PushedAt          string                `json:"pushedAt,omitempty"`
	RecommendedAction string                `json:"recommendedAction"`
}

func newStatusCmd() *cobra.Command {
	var f statusFlags
	cmd := &cobra.Command{
		Use:   "status <date>[/<name>]",
		Short: "Show one-line draft status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	cmd.Flags().BoolVar(&f.noRemote, "no-remote-check", false, "skip remote fingerprint probe")
	return cmd
}

func runStatus(cmd *cobra.Command, f statusFlags, ref string) error {
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

	pulled, _ := drafts.PulledCellsByKey(profileName, weekStart, name)
	fingerprint := ""
	if !f.noRemote {
		fingerprint = drafts.ProbeRemoteFingerprint(cmd.Context(), profileName, weekStart)
	}
	state := domain.ComputeSyncState(d, pulled, fingerprint)
	action := recommendedAction(state.Sync, state.Stale)

	return renderStatus(cmd.OutOrStdout(), d, state, action, f.json)
}

func recommendedAction(sync domain.SyncState, stale bool) string {
	switch {
	case sync == domain.SyncConflicted:
		return "edit to resolve conflicts (refresh available in Phase B)"
	case sync == domain.SyncDirty && stale:
		return "remote drifted since pull; tdx time week pull --force <date> (auto-snapshots) before pushing"
	case sync == domain.SyncDirty:
		return "tdx time week preview <date>, then push --yes"
	case sync == domain.SyncClean && stale:
		return "tdx time week pull <date> (will adopt remote changes)"
	default:
		return "no action recommended"
	}
}

func renderStatus(w io.Writer, d domain.WeekDraft, state domain.DraftSyncState, action string, jsonOut bool) error {
	if jsonOut {
		return json.NewEncoder(w).Encode(weekDraftStatusResp{
			Schema:            "tdx.v1.weekDraftStatus",
			Profile:           d.Profile,
			WeekStart:         d.WeekStart.Format("2006-01-02"),
			Name:              d.Name,
			SyncState:         string(state.Sync),
			SyncDetail:        state,
			TotalHours:        state.TotalHours,
			PulledAt:          formatRFC3339OrEmpty(d.Provenance.PulledAt),
			PushedAt:          formatPtrRFC3339OrEmpty(d.PushedAt),
			RecommendedAction: action,
		})
	}
	fmt.Fprintf(w, "%s / %s\n", d.WeekStart.Format("2006-01-02"), d.Name)
	fmt.Fprintf(w, "  Profile:     %s\n", d.Profile)
	fmt.Fprintf(w, "  Pulled:      %s\n", formatTimeWithAge(d.Provenance.PulledAt))
	fmt.Fprintf(w, "  Pushed:      %s\n", formatPtrTimeWithAge(d.PushedAt))
	syncLabel := string(state.Sync)
	if state.Stale {
		syncLabel += " (and STALE)"
	}
	fmt.Fprintf(w, "  Sync state:  %s\n", syncLabel)
	fmt.Fprintf(w, "  Cells:       %d untouched · %d edited · %d added · %d conflict\n",
		state.Untouched, state.Edited, state.Added, state.Conflict)
	fmt.Fprintf(w, "  Total hours: %.1fh\n\n", state.TotalHours)
	fmt.Fprintf(w, "  Action recommended:\n    %s\n", action)
	return nil
}

func formatPtrRFC3339OrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatRFC3339OrEmpty(*t)
}

func formatTimeWithAge(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return fmt.Sprintf("%s (%s ago)",
		t.UTC().Format("2006-01-02 15:04:05"),
		time.Since(t).Round(time.Minute))
}

func formatPtrTimeWithAge(t *time.Time) string {
	if t == nil {
		return "never"
	}
	return formatTimeWithAge(*t)
}
