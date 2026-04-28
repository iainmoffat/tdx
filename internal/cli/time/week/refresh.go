package week

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type refreshFlags struct {
	profile  string
	strategy string
	json     bool
}

func newRefreshCmd() *cobra.Command {
	var f refreshFlags
	cmd := &cobra.Command{
		Use:   "refresh <date>[/<name>]",
		Short: "Three-way merge a draft against the latest remote",
		Long: `Refresh re-fetches the live week and merges remote changes into the local draft.

  --strategy abort    (default) refuse to mutate if any cell-level conflict
  --strategy ours     on conflict, keep local
  --strategy theirs   on conflict, take remote

On --strategy abort with conflicts, refresh exits non-zero and prints the
list of conflicts. The local draft is unchanged. Re-run with --strategy ours
or --strategy theirs to proceed, or 'tdx time week reset --yes' to discard
local edits and re-pull.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefresh(cmd, f, args[0])
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name")
	cmd.Flags().StringVar(&f.strategy, "strategy", "abort", "abort | ours | theirs")
	cmd.Flags().BoolVar(&f.json, "json", false, "JSON output")
	return cmd
}

func runRefresh(cmd *cobra.Command, f refreshFlags, ref string) error {
	weekStart, name, err := ParseDraftRef(ref)
	if err != nil {
		return err
	}
	strategy := draftsvc.Strategy(f.strategy)
	if err := strategy.Validate(); err != nil {
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

	res, err := drafts.Refresh(cmd.Context(), profileName, weekStart, name, strategy)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	if f.json {
		return writeRefreshJSON(w, weekStart, name, res)
	}
	if res.Aborted {
		writeRefreshAbortText(w, weekStart, res)
		return fmt.Errorf("refresh aborted: %d conflict(s)", len(res.Conflicts))
	}
	writeRefreshSuccessText(w, res)
	return nil
}

func writeRefreshSuccessText(w io.Writer, res draftsvc.RefreshResult) {
	suffix := ""
	if res.Strategy != draftsvc.StrategyAbort {
		suffix = fmt.Sprintf(" (--strategy %s)", res.Strategy)
	}
	_, _ = fmt.Fprintf(w, "Refresh complete%s.\n", suffix)
	_, _ = fmt.Fprintf(w, "  Adopted (remote -> draft):  %d cells\n", res.Adopted)
	_, _ = fmt.Fprintf(w, "  Preserved (local edits):    %d cells\n", res.Preserved)
	_, _ = fmt.Fprintf(w, "  Resolved (same on both):    %d cells\n", res.Resolved)
	if res.ResolvedByStrategy > 0 {
		who := "local won"
		if res.Strategy == draftsvc.StrategyTheirs {
			who = "remote won"
		}
		_, _ = fmt.Fprintf(w, "  Resolved by --strategy:     %d cells (%s)\n",
			res.ResolvedByStrategy, who)
	}
}

func writeRefreshAbortText(w io.Writer, weekStart time.Time, res draftsvc.RefreshResult) {
	_, _ = fmt.Fprintf(w, "Refresh aborted: %d cell(s) conflict between local edits and remote changes.\n\n",
		len(res.Conflicts))
	for _, c := range res.Conflicts {
		_, _ = fmt.Fprintf(w, "  %s  %s  conflict\n", c.RowID, c.Day[:3])
		_, _ = fmt.Fprintf(w, "    local:   %s\n", c.LocalDescription)
		_, _ = fmt.Fprintf(w, "    remote:  %s\n", c.RemoteDescription)
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintln(w, "Choose one:")
	_, _ = fmt.Fprintln(w, "  --strategy ours        (keep local for all conflicts; refresh succeeds)")
	_, _ = fmt.Fprintln(w, "  --strategy theirs      (take remote for all conflicts; refresh succeeds)")
	_, _ = fmt.Fprintf(w, "  tdx time week reset %s --yes  (give up local edits entirely, re-pull fresh)\n",
		weekStart.Format("2006-01-02"))
}

func writeRefreshJSON(w io.Writer, weekStart time.Time, name string, res draftsvc.RefreshResult) error {
	// Implemented in Task 13.
	_ = weekStart
	_ = name
	_ = res
	_, _ = io.WriteString(w, "{}\n")
	return nil
}
