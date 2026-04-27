package week

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type diffFlags struct {
	profile string
	against string
	json    bool
}

type weekDraftDiffEntry struct {
	Row      string  `json:"row"`
	Day      string  `json:"day"`
	Kind     string  `json:"kind"`
	Before   float64 `json:"before"`
	After    float64 `json:"after"`
	SourceID int     `json:"sourceID,omitempty"`
}

type weekDraftDiffSummary struct {
	Adds    int `json:"adds"`
	Updates int `json:"updates"`
	Deletes int `json:"deletes"`
	Matches int `json:"matches"`
}

type weekDraftDiffResp struct {
	Schema  string               `json:"schema"`
	Entries []weekDraftDiffEntry `json:"entries"`
	Summary weekDraftDiffSummary `json:"summary"`
}

func newDiffCmd() *cobra.Command {
	var f diffFlags
	cmd := &cobra.Command{
		Use:   "diff <date>[/<name>]",
		Short: "Diff a draft against current remote",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().StringVar(&f.against, "against", "remote", "diff target (MVP supports \"remote\" only; Phase D adds template/snapshot/draft)")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runDiff(cmd *cobra.Command, f diffFlags, ref string) error {
	if f.against != "" && f.against != "remote" {
		return fmt.Errorf("--against %q not supported in Phase A (only \"remote\"; Phase D adds template/snapshot/draft)", f.against)
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

	user, err := auth.WhoAmI(cmd.Context(), profileName)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}

	_, diff, err := drafts.Reconcile(cmd.Context(), profileName, weekStart, name, user.UID)
	if err != nil {
		return err
	}

	return renderDiff(cmd.OutOrStdout(), diff, f.json)
}

func renderDiff(w io.Writer, diff domain.ReconcileDiff, jsonOut bool) error {
	entries := make([]weekDraftDiffEntry, 0, len(diff.Actions))
	var summary weekDraftDiffSummary
	for _, a := range diff.Actions {
		e := weekDraftDiffEntry{Row: a.RowID, Day: a.Date.Weekday().String()}
		switch a.Kind {
		case domain.ActionCreate:
			e.Kind = "add"
			e.After = float64(a.Entry.Minutes) / 60.0
			summary.Adds++
		case domain.ActionUpdate:
			e.Kind = "update"
			e.SourceID = a.ExistingID
			if a.Patch.Minutes != nil {
				e.After = float64(*a.Patch.Minutes) / 60.0
			}
			summary.Updates++
		case domain.ActionDelete:
			e.Kind = "delete"
			e.SourceID = a.DeleteEntryID
			summary.Deletes++
		case domain.ActionSkip:
			e.Kind = "match"
			e.SourceID = a.ExistingID
			summary.Matches++
		}
		entries = append(entries, e)
	}

	if jsonOut {
		return json.NewEncoder(w).Encode(weekDraftDiffResp{
			Schema:  "tdx.v1.weekDraftDiff",
			Entries: entries,
			Summary: summary,
		})
	}

	// Default text view: one line per non-match action.
	for _, e := range entries {
		if e.Kind == "match" {
			continue
		}
		fmt.Fprintf(w, "  %-8s  %-3s  %-7s  %.1f -> %.1f", e.Row, e.Day[:3], e.Kind, e.Before, e.After)
		if e.SourceID > 0 {
			fmt.Fprintf(w, "  (entry #%d)", e.SourceID)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "\nSummary: %d adds · %d updates · %d deletes · %d matches\n",
		summary.Adds, summary.Updates, summary.Deletes, summary.Matches)
	return nil
}
