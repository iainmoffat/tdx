package week

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type pushFlags struct {
	profile          string
	yes              bool
	allowDeletes     bool
	expectedDiffHash string
	json             bool
}

type weekDraftPushResp struct {
	Schema  string                  `json:"schema"`
	Created int                     `json:"created"`
	Updated int                     `json:"updated"`
	Deleted int                     `json:"deleted"`
	Skipped int                     `json:"skipped"`
	Failed  []draftsvc.ApplyFailure `json:"failed,omitempty"`
}

func newPushCmd() *cobra.Command {
	var f pushFlags
	cmd := &cobra.Command{
		Use:   "push <date>[/<name>]",
		Short: "Push a draft to TD",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPush(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "execute the push (otherwise behaves as preview)")
	cmd.Flags().BoolVar(&f.allowDeletes, "allow-deletes", false, "required if any delete actions in preview")
	cmd.Flags().StringVar(&f.expectedDiffHash, "expected-diff-hash", "", "use a specific diff hash (advanced; default re-computes)")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runPush(cmd *cobra.Command, f pushFlags, ref string) error {
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

	// Always preview first.
	_, diff, err := drafts.Reconcile(cmd.Context(), profileName, weekStart, name, user.UID)
	if err != nil {
		return err
	}
	creates, updates, deletes, skips := diff.CountByKindV2()

	if !f.yes {
		// Identical to preview when --yes is not set.
		return renderPreview(cmd.OutOrStdout(), diff, creates, updates, deletes, skips, f.json)
	}
	if deletes > 0 && !f.allowDeletes {
		return fmt.Errorf("draft contains %d delete actions; pass --allow-deletes to confirm", deletes)
	}

	hash := diff.DiffHash
	if f.expectedDiffHash != "" {
		hash = f.expectedDiffHash
	}

	res, err := drafts.Apply(cmd.Context(), profileName, weekStart, name, hash, f.allowDeletes, user.UID)
	if err != nil {
		if strings.Contains(err.Error(), "hash mismatch") {
			return fmt.Errorf(
				"push aborted: remote week changed since preview\n  Re-run: tdx time week preview %s\n  Or:    tdx time week pull --force %s (auto-snapshots)",
				weekStart.Format("2006-01-02"), weekStart.Format("2006-01-02"))
		}
		return err
	}
	return renderPushResult(cmd.OutOrStdout(), res, f.json)
}

func renderPushResult(w io.Writer, res draftsvc.ApplyResult, jsonOut bool) error {
	if jsonOut {
		return json.NewEncoder(w).Encode(weekDraftPushResp{
			Schema:  "tdx.v1.weekDraftPushResult",
			Created: res.Created,
			Updated: res.Updated,
			Deleted: res.Deleted,
			Skipped: res.Skipped,
			Failed:  res.Failed,
		})
	}
	fmt.Fprintf(w, "Push complete: %d created · %d updated · %d deleted · %d skipped\n",
		res.Created, res.Updated, res.Deleted, res.Skipped)
	if len(res.Failed) > 0 {
		fmt.Fprintf(w, "\nFailures (%d):\n", len(res.Failed))
		for _, fail := range res.Failed {
			id := ""
			if fail.EntryID > 0 {
				id = fmt.Sprintf(" (entry #%d)", fail.EntryID)
			}
			fmt.Fprintf(w, "  %s %s/%s: %s%s\n", fail.Kind, fail.RowID, fail.Date, fail.Message, id)
		}
	}
	return nil
}
