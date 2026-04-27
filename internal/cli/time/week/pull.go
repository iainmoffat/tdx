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

type pullFlags struct {
	profile string
	name    string
	force   bool
	json    bool
}

// weekDraftPullResult is the JSON envelope for `tdx time week pull`.
// Schema: tdx.v1.weekDraftPullResult.
type weekDraftPullResult struct {
	Schema string           `json:"schema"`
	Draft  domain.WeekDraft `json:"draft"`
}

func newPullCmd() *cobra.Command {
	var f pullFlags
	cmd := &cobra.Command{
		Use:   "pull <date>",
		Short: "Pull a live week into a local draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPull(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name (defaults to active profile)")
	cmd.Flags().StringVar(&f.name, "name", "", `draft name (defaults to "default")`)
	cmd.Flags().BoolVar(&f.force, "force", false, "overwrite a dirty draft (auto-snapshots first)")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runPull(cmd *cobra.Command, f pullFlags, ref string) error {
	weekStart, name, err := ParseDraftRef(ref)
	if err != nil {
		return err
	}
	if f.name != "" {
		name = f.name
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

	draft, err := drafts.Pull(cmd.Context(), profileName, weekStart, name, f.force)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	if f.json {
		return writePullResultJSON(w, draft)
	}
	writePullResultText(w, draft)
	return nil
}

// writePullResultJSON encodes the pull result as a tdx.v1.weekDraftPullResult JSON envelope.
func writePullResultJSON(w io.Writer, d domain.WeekDraft) error {
	return json.NewEncoder(w).Encode(weekDraftPullResult{
		Schema: "tdx.v1.weekDraftPullResult",
		Draft:  d,
	})
}

// writePullResultText writes a human-readable summary and next-step hints.
func writePullResultText(w io.Writer, d domain.WeekDraft) {
	var totalCells int
	var totalHours float64
	for _, row := range d.Rows {
		for _, cell := range row.Cells {
			totalCells++
			totalHours += cell.Hours
		}
	}
	fmt.Fprintf(w, "Created draft %s/%s (%d rows, %d cells, %.1fh, status: %s)\n\n",
		d.WeekStart.Format("2006-01-02"), d.Name, len(d.Rows), totalCells, totalHours, d.Provenance.RemoteStatus)
	fmt.Fprintf(w, "  tdx time week show %s --draft     # view the draft\n", d.WeekStart.Format("2006-01-02"))
	fmt.Fprintf(w, "  tdx time week edit %s             # edit it\n", d.WeekStart.Format("2006-01-02"))
}
