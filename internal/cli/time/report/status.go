package report

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/spf13/cobra"
)

type statusFlags struct {
	profile       string
	week          string
	from          string
	to            string
	users         []string
	managers      []string
	accounts      []string
	resourcePools []string
	all           bool
	yes           bool
	includeZero   bool
	incomplete    bool
	threshold     float64
	thresholdSet  bool // true when --threshold was explicitly passed (CLI: cmd.Flags().Changed("threshold"); MCP: in.Threshold > 0)
	limit         int
	json          bool
	csv           bool
	xlsx          string
}

func newStatusCmd() *cobra.Command {
	var f statusFlags
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Weekly time-status report (per user, per week)",
		Long: `Render TeamDynamix's "Time Status Report" (Work Management → Analysis → Standard Reports).

For each (user, week) pair, prints submission status and billable / non-billable / total hours.

Selectors (exactly one selector type required; each accepts repeated /
comma-separated values, treated as a union):
  --user UID          one or more user UIDs
  --manager UID       direct reports of one or more manager UIDs (use "me" for the authenticated user)
  --account NAME      users in one or more accounts/departments by name
  --resource-pool NAME users in one or more TD resource pools by name
  --all               every active employee (requires --yes)

Filters:
  --incomplete       keep only user-weeks under --threshold (drops permission-denied)
  --threshold N      hours threshold for --incomplete (default 40)

Output formats (mutually exclusive; default: human table):
  --json       JSON envelope on stdout
  --csv        CSV on stdout (no subtotal rows; pivot in Excel)
  --xlsx PATH  write XLSX to PATH`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("threshold") && !f.incomplete {
				return fmt.Errorf("--threshold requires --incomplete")
			}
			f.thresholdSet = cmd.Flags().Changed("threshold")
			if err := validateStatusFlags(f); err != nil {
				return err
			}
			return runStatus(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name (default: active)")
	cmd.Flags().StringVar(&f.week, "week", "", "any date inside the target week (YYYY-MM-DD)")
	cmd.Flags().StringVar(&f.from, "from", "", "range start (YYYY-MM-DD); requires --to")
	cmd.Flags().StringVar(&f.to, "to", "", "range end (YYYY-MM-DD); requires --from")
	cmd.Flags().StringSliceVar(&f.users, "user", nil, "user UIDs (repeatable / comma-separated)")
	cmd.Flags().StringSliceVar(&f.managers, "manager", nil, "manager UIDs (repeatable / comma-separated; \"me\" = authenticated user)")
	cmd.Flags().StringSliceVar(&f.accounts, "account", nil, "account/department names (repeatable / comma-separated)")
	cmd.Flags().StringSliceVar(&f.resourcePools, "resource-pool", nil, "TD resource pool names (repeatable / comma-separated)")
	cmd.Flags().BoolVar(&f.all, "all", false, "every active employee (requires --yes)")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "confirm --all")
	cmd.Flags().BoolVar(&f.includeZero, "include-zero", true, "include user-weeks with zero total minutes (default: include)")
	cmd.Flags().BoolVar(&f.incomplete, "incomplete", false, "filter to user-weeks below --threshold (excludes permission-denied rows)")
	cmd.Flags().Float64Var(&f.threshold, "threshold", 40, "hours threshold for --incomplete (default 40)")
	cmd.Flags().IntVar(&f.limit, "limit", 200, "cap user count (hard cap: 1000)")
	cmd.Flags().BoolVar(&f.json, "json", false, "emit JSON to stdout")
	cmd.Flags().BoolVar(&f.csv, "csv", false, "emit CSV to stdout")
	cmd.Flags().StringVar(&f.xlsx, "xlsx", "", "write XLSX to this file path")
	return cmd
}

// validateStatusFlags enforces selector + format exclusivity rules
// before any API calls happen.
func validateStatusFlags(f statusFlags) error {
	// Selectors — count distinct selector *types* in use, not individual values.
	selectors := 0
	if len(f.users) > 0 {
		selectors++
	}
	if len(f.managers) > 0 {
		selectors++
	}
	if len(f.accounts) > 0 {
		selectors++
	}
	if len(f.resourcePools) > 0 {
		selectors++
	}
	if f.all {
		selectors++
	}
	switch {
	case selectors == 0:
		return fmt.Errorf("a selector is required: --user, --manager, --account, --resource-pool, or --all")
	case selectors > 1:
		return fmt.Errorf("exactly one of --user, --manager, --account, --resource-pool, --all may be set")
	}

	if f.all && !f.yes {
		return fmt.Errorf("--all is destructively large; pass --yes to confirm")
	}

	// Formats
	formats := 0
	if f.json {
		formats++
	}
	if f.csv {
		formats++
	}
	if f.xlsx != "" {
		formats++
	}
	if formats > 1 {
		return fmt.Errorf("only one output format flag may be set (--json, --csv, --xlsx)")
	}

	// Limit cap
	if f.limit > 1000 {
		return fmt.Errorf("--limit cannot exceed 1000")
	}

	// Date range
	if f.week != "" && (f.from != "" || f.to != "") {
		return fmt.Errorf("--week is mutually exclusive with --from/--to")
	}
	if (f.from != "") != (f.to != "") {
		return fmt.Errorf("--from and --to must be given together")
	}
	if f.week == "" && f.from == "" {
		return fmt.Errorf("--week or --from/--to is required")
	}

	return nil
}

func runStatus(cmd *cobra.Command, f statusFlags) error {
	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	profile, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}

	deps := runnerDeps{
		Time:    timesvc.New(paths),
		People:  peoplesvc.New(paths),
		Auth:    auth,
		Profile: profile,
	}

	rep, err := assembleReport(cmd.Context(), deps, f)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	switch {
	case f.json:
		return printJSON(w, rep, f)
	case f.csv:
		return writeCSV(w, rep)
	case f.xlsx != "":
		return writeXLSX(f.xlsx, rep)
	default:
		return printText(w, rep, f)
	}
}
