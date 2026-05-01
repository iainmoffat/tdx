package week

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type resolveFlags struct {
	profile   string
	row       string
	day       string
	pick      string
	allLocal  bool
	allRemote bool
	yes       bool
	json      bool
}

func newResolveCmd() *cobra.Command {
	var f resolveFlags
	cmd := &cobra.Command{
		Use:   "resolve [date[/name]]",
		Short: "Pick winners for cell-level conflicts produced by --strategy surface",
		Long: `Resolve produces one of three behaviors:

  bare                                     prints the conflict list
  --all-local / --all-remote               applies the same choice to every conflict
  --row ID --day NAME --pick local|remote  applies one per-cell pick

--pick remote on a cell whose remote candidate is "delete" requires --yes.

The flag triple (--row, --day, --pick) is required together; none is allowed
alone. --all-local / --all-remote are mutually exclusive with each other and
with the per-cell triple.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) > 0 {
				ref = args[0]
			}
			return runResolve(cmd, f, ref)
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().StringVar(&f.row, "row", "", "row ID for a per-cell pick")
	cmd.Flags().StringVar(&f.day, "day", "", "weekday name for a per-cell pick (e.g. Monday)")
	cmd.Flags().StringVar(&f.pick, "pick", "", "local | remote")
	cmd.Flags().BoolVar(&f.allLocal, "all-local", false, "resolve every conflict by keeping local")
	cmd.Flags().BoolVar(&f.allRemote, "all-remote", false, "resolve every conflict by taking remote")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "confirm picks that drop cells (remote candidate = delete)")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runResolve(cmd *cobra.Command, f resolveFlags, ref string) error {
	if err := validateResolveFlags(f); err != nil {
		return err
	}

	weekStart, name, err := ResolveWeekRef(ref)
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

	w := cmd.OutOrStdout()

	// Bare invocation (no apply flags) → status output.
	if !f.allLocal && !f.allRemote && f.row == "" {
		conflicts, err := drafts.ListConflicts(profileName, weekStart, name)
		if err != nil {
			return err
		}
		if f.json {
			return writeConflictsJSON(w, weekStart, name, conflicts)
		}
		writeConflictsText(w, weekStart, conflicts)
		return nil
	}

	// Apply path.
	var picks []draftsvc.Pick
	if f.row != "" {
		day, err := draftsvc.ParseWeekday(f.day)
		if err != nil {
			return err
		}
		choice, err := draftsvc.ParsePickChoice(f.pick)
		if err != nil {
			return err
		}
		picks = []draftsvc.Pick{{RowID: f.row, Day: day, Choice: choice}}
	}

	res, err := drafts.Resolve(profileName, weekStart, name, f.allLocal, f.allRemote, picks, f.yes)
	if err != nil {
		return err
	}

	if f.json {
		return writeResolveJSON(w, weekStart, name, res)
	}
	writeResolveText(w, weekStart, res)
	return nil
}

func validateResolveFlags(f resolveFlags) error {
	tripleSet := f.row != "" || f.day != "" || f.pick != ""
	tripleComplete := f.row != "" && f.day != "" && f.pick != ""
	bulk := f.allLocal || f.allRemote

	if tripleSet && !tripleComplete {
		return fmt.Errorf("--row, --day, and --pick must be given together")
	}
	if f.allLocal && f.allRemote {
		return fmt.Errorf("--all-local and --all-remote are mutually exclusive")
	}
	if bulk && tripleComplete {
		return fmt.Errorf("--all-local/--all-remote are mutually exclusive with --row/--day/--pick")
	}
	return nil
}

func writeConflictsText(w io.Writer, weekStart time.Time, conflicts []draftsvc.Conflict) {
	if len(conflicts) == 0 {
		_, _ = fmt.Fprintln(w, "no conflicts to resolve")
		return
	}
	_, _ = fmt.Fprintf(w, "WEEK %s  conflicts: %d\n", weekStart.Format("2006-01-02"), len(conflicts))
	headers := []string{"ROW", "DAY", "LOCAL", "REMOTE", "PULLED"}
	rows := make([][]string, 0, len(conflicts))
	for _, c := range conflicts {
		rows = append(rows, []string{
			c.RowID,
			c.Day.String(),
			describeConflictSide(c.LocalHours, c.LocalSrcID),
			describeConflictSide(c.RemoteHours, c.RemoteSrcID),
			fmt.Sprintf("%.1f", c.PulledHours),
		})
	}
	render.Table(w, headers, rows, nil)
	_, _ = fmt.Fprintln(w, "\nPick:")
	if len(conflicts) > 0 {
		_, _ = fmt.Fprintf(w, "  tdx time week resolve %s --row %s --day %s --pick remote\n",
			weekStart.Format("2006-01-02"), conflicts[0].RowID, conflicts[0].Day.String())
	}
	_, _ = fmt.Fprintf(w, "  tdx time week resolve %s --all-local\n", weekStart.Format("2006-01-02"))
	_, _ = fmt.Fprintf(w, "  tdx time week resolve %s --all-remote --yes\n", weekStart.Format("2006-01-02"))
}

func describeConflictSide(hours float64, sourceEntryID int) string {
	if hours == 0 && sourceEntryID == 0 {
		return "(deleted)"
	}
	if hours == 0 {
		return "(cleared)"
	}
	return fmt.Sprintf("%.1f", hours)
}

func writeResolveText(w io.Writer, weekStart time.Time, res draftsvc.ResolveResult) {
	_, _ = fmt.Fprintf(w, "Resolved %d conflict(s) on %s.\n",
		res.PicksApplied, weekStart.Format("2006-01-02"))
	_, _ = fmt.Fprintf(w, "  Picked local:  %d\n", res.PickedLocal)
	_, _ = fmt.Fprintf(w, "  Picked remote: %d\n", res.PickedRemote)
	if res.DroppedDeletedCells > 0 {
		_, _ = fmt.Fprintf(w, "  Dropped cells: %d (remote intent: delete)\n", res.DroppedDeletedCells)
	}
	_, _ = fmt.Fprintf(w, "  Conflicts remaining: %d\n", res.ConflictsRemaining)
	if res.ConflictsRemaining == 0 && res.PicksApplied > 0 {
		_, _ = fmt.Fprintf(w, "\ntdx time week preview %s, then push --yes\n",
			weekStart.Format("2006-01-02"))
	}
}

type conflictJSON struct {
	RowID         string  `json:"rowID"`
	RowLabel      string  `json:"rowLabel,omitempty"`
	Day           string  `json:"day"`
	LocalHours    float64 `json:"localHours"`
	LocalSrcID    int     `json:"localSourceEntryID,omitempty"`
	RemoteHours   float64 `json:"remoteHours"`
	RemoteSrcID   int     `json:"remoteSourceEntryID,omitempty"`
	PulledHours   float64 `json:"pulledHours"`
	RemoteDeletes bool    `json:"remoteDeletes,omitempty"`
}

func writeConflictsJSON(w io.Writer, weekStart time.Time, name string, conflicts []draftsvc.Conflict) error {
	out := make([]conflictJSON, 0, len(conflicts))
	for _, c := range conflicts {
		out = append(out, conflictJSON{
			RowID: c.RowID, RowLabel: c.RowLabel,
			Day:           c.Day.String(),
			LocalHours:    c.LocalHours,
			LocalSrcID:    c.LocalSrcID,
			RemoteHours:   c.RemoteHours,
			RemoteSrcID:   c.RemoteSrcID,
			PulledHours:   c.PulledHours,
			RemoteDeletes: c.RemoteDeletes,
		})
	}
	envelope := struct {
		Schema    string         `json:"schema"`
		WeekStart string         `json:"weekStart"`
		Name      string         `json:"name"`
		Conflicts []conflictJSON `json:"conflicts"`
	}{
		Schema:    "tdx.v1.weekDraftConflicts",
		WeekStart: weekStart.Format("2006-01-02"),
		Name:      name,
		Conflicts: out,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func writeResolveJSON(w io.Writer, weekStart time.Time, name string, res draftsvc.ResolveResult) error {
	envelope := struct {
		Schema              string `json:"schema"`
		WeekStart           string `json:"weekStart"`
		Name                string `json:"name"`
		PicksApplied        int    `json:"picksApplied"`
		PickedLocal         int    `json:"pickedLocal"`
		PickedRemote        int    `json:"pickedRemote"`
		DroppedDeletedCells int    `json:"droppedDeletedCells"`
		ConflictsRemaining  int    `json:"conflictsRemaining"`
	}{
		Schema:              "tdx.v1.weekDraftResolveResult",
		WeekStart:           weekStart.Format("2006-01-02"),
		Name:                name,
		PicksApplied:        res.PicksApplied,
		PickedLocal:         res.PickedLocal,
		PickedRemote:        res.PickedRemote,
		DroppedDeletedCells: res.DroppedDeletedCells,
		ConflictsRemaining:  res.ConflictsRemaining,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
