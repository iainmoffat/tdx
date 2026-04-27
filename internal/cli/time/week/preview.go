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

type previewFlags struct {
	profile string
	json    bool
}

type weekDraftPreviewResp struct {
	Schema           string           `json:"schema"`
	Actions          []domain.Action  `json:"actions"`
	Blockers         []domain.Blocker `json:"blockers"`
	Creates          int              `json:"creates"`
	Updates          int              `json:"updates"`
	Deletes          int              `json:"deletes"`
	Skips            int              `json:"skips"`
	BlockedCount     int              `json:"blockedCount"`
	ExpectedDiffHash string           `json:"expectedDiffHash"`
}

func newPreviewCmd() *cobra.Command {
	var f previewFlags
	cmd := &cobra.Command{
		Use:   "preview <date>[/<name>]",
		Short: "Preview what tdx time week push will do",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPreview(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runPreview(cmd *cobra.Command, f previewFlags, ref string) error {
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

	creates, updates, deletes, skips := diff.CountByKindV2()
	return renderPreview(cmd.OutOrStdout(), diff, creates, updates, deletes, skips, f.json)
}

func renderPreview(w io.Writer, diff domain.ReconcileDiff, creates, updates, deletes, skips int, jsonOut bool) error {
	if jsonOut {
		return json.NewEncoder(w).Encode(weekDraftPreviewResp{
			Schema:           "tdx.v1.weekDraftPreview",
			Actions:          diff.Actions,
			Blockers:         diff.Blockers,
			Creates:          creates,
			Updates:          updates,
			Deletes:          deletes,
			Skips:            skips,
			BlockedCount:     len(diff.Blockers),
			ExpectedDiffHash: diff.DiffHash,
		})
	}

	// Text view: list non-match actions, then summary + hash.
	for _, a := range diff.Actions {
		if a.Kind == domain.ActionSkip {
			continue
		}
		kind := a.Kind.String()
		line := fmt.Sprintf("  %-8s  %-3s  %-8s", a.RowID, a.Date.Weekday().String()[:3], kind)
		switch a.Kind {
		case domain.ActionCreate:
			line += fmt.Sprintf("  %.1fh", float64(a.Entry.Minutes)/60.0)
		case domain.ActionUpdate:
			if a.Patch.Minutes != nil {
				line += fmt.Sprintf("  -> %.1fh  (entry #%d)", float64(*a.Patch.Minutes)/60.0, a.ExistingID)
			} else {
				line += fmt.Sprintf("  (entry #%d)", a.ExistingID)
			}
		case domain.ActionDelete:
			line += fmt.Sprintf("  (entry #%d)", a.DeleteEntryID)
		}
		fmt.Fprintln(w, line)
	}
	for _, b := range diff.Blockers {
		fmt.Fprintf(w, "  %-8s  %-3s  blocked   %s (%s)\n",
			b.RowID, b.Date.Weekday().String()[:3], b.Kind, b.Reason)
	}
	fmt.Fprintf(w, "\nSummary: %d creates · %d updates · %d deletes · %d skips · %d blocked\n",
		creates, updates, deletes, skips, len(diff.Blockers))
	if len(diff.DiffHash) >= 16 {
		fmt.Fprintf(w, "Diff hash: %s...\n", diff.DiffHash[:16])
	} else {
		fmt.Fprintf(w, "Diff hash: %s\n", diff.DiffHash)
	}
	return nil
}
