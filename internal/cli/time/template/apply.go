package template

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
)

// ErrPartialApply signals that some entries were created but others failed.
// The CLI uses this to exit with code 2.
type ErrPartialApply struct {
	Created int
	Failed  int
	Message string
}

func (e *ErrPartialApply) Error() string { return e.Message }

func newApplyCmd() *cobra.Command {
	var (
		weekFlag    string
		modeFlag    string
		daysFlag    string
		overrides   []string
		roundFlag   bool
		dryRun      bool
		yesFlag     bool
		jsonFlag    bool
		profileFlag string
	)

	cmd := &cobra.Command{
		Use:   "apply <name>",
		Short: "Apply a template to create time entries for a week",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// --week is required.
			if weekFlag == "" {
				return fmt.Errorf("--week is required")
			}
			weekDate, err := time.ParseInLocation("2006-01-02", weekFlag, domain.EasternTZ)
			if err != nil {
				return fmt.Errorf("invalid --week: %w", err)
			}

			// Parse --mode.
			mode, err := domain.ParseApplyMode(modeFlag)
			if err != nil {
				return err
			}

			// Parse --days.
			var daysFilter []time.Weekday
			if daysFlag != "" {
				daysFilter, err = parseDays(daysFlag)
				if err != nil {
					return fmt.Errorf("invalid --days: %w", err)
				}
			}

			// Parse --override flags.
			var parsedOverrides []tmplsvc.Override
			for _, o := range overrides {
				ov, err := parseOverride(o)
				if err != nil {
					return fmt.Errorf("invalid --override %q: %w", o, err)
				}
				parsedOverrides = append(parsedOverrides, ov)
			}

			// Wire up services.
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			tsvc := timesvc.New(paths)
			svc := tmplsvc.New(paths, tsvc)

			profileName, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}

			// Load template.
			tmpl, err := svc.Store().Load(profileName, name)
			if err != nil {
				return err
			}

			// Resolve week.
			weekRef := domain.WeekRefContaining(weekDate)

			// Get user identity.
			user, err := auth.WhoAmI(cmd.Context(), profileName)
			if err != nil {
				return err
			}

			// Build reconcile input.
			input := tmplsvc.ReconcileInput{
				Template:   tmpl,
				WeekRef:    weekRef,
				Mode:       mode,
				DaysFilter: daysFilter,
				Overrides:  parsedOverrides,
				Round:      roundFlag,
				UserUID:    user.UID,
			}

			// Run reconciliation preview.
			diff, err := svc.Reconcile(cmd.Context(), profileName, input)
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			creates, updates, skips := diff.CountByKind()
			blockers := len(diff.Blockers)

			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})

			if format == render.FormatJSON {
				return render.JSON(w, struct {
					Schema      string           `json:"schema"`
					DiffHash    string           `json:"diffHash"`
					Creates     int              `json:"creates"`
					Updates     int              `json:"updates"`
					Skips       int              `json:"skips"`
					Blockers    int              `json:"blockers"`
					Actions     []domain.Action  `json:"actions"`
					BlockerList []domain.Blocker `json:"blockerList,omitempty"`
				}{
					Schema:      "tdx.v1.templateApplyPreview",
					DiffHash:    diff.DiffHash,
					Creates:     creates,
					Updates:     updates,
					Skips:       skips,
					Blockers:    blockers,
					Actions:     diff.Actions,
					BlockerList: diff.Blockers,
				})
			}

			// Render annotated grid preview.
			render.Grid(w, diffToGridData(tmpl, diff))

			// Summary line.
			_, _ = fmt.Fprintf(w, "\n%d to create, %d to update, %d skipped, %d blocked\n",
				creates, updates, skips, blockers)

			// Dry-run: stop here.
			if dryRun {
				return nil
			}

			// If neither --yes nor --dry-run, instruct user.
			if !yesFlag {
				_, _ = fmt.Fprintf(w, "\nUse --yes to apply, or --dry-run to preview without changes.\n")
				return nil
			}

			// Apply.
			result, err := svc.Apply(cmd.Context(), profileName, input, diff.DiffHash)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(w, "\napplied: %d created, %d updated, %d skipped",
				result.Created, result.Updated, result.Skipped)
			if len(result.Failed) > 0 {
				_, _ = fmt.Fprintf(w, ", %d failed", len(result.Failed))
			}
			_, _ = fmt.Fprintln(w)

			// Report individual failures.
			for _, f := range result.Failed {
				_, _ = fmt.Fprintf(w, "  FAIL %s %s: %s\n", f.RowID, f.Date, f.Message)
			}

			if len(result.Failed) > 0 {
				return &ErrPartialApply{
					Created: result.Created,
					Failed:  len(result.Failed),
					Message: fmt.Sprintf("partial apply: %d created, %d failed", result.Created, len(result.Failed)),
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&weekFlag, "week", "", "target week date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&modeFlag, "mode", "add", "apply mode: add, replace-matching, replace-mine")
	cmd.Flags().StringVar(&daysFlag, "days", "", "filter days: range (mon-fri) or list (mon,wed,fri)")
	cmd.Flags().StringArrayVar(&overrides, "override", nil, "override row hours: row-id:day=hours (repeatable)")
	cmd.Flags().BoolVar(&roundFlag, "round", false, "round fractional minutes to nearest whole minute")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without applying")
	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "apply without confirmation prompt")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

// diffToGridData converts a reconciliation diff + source template into
// annotated GridData for the grid renderer. Each cell is marked with the
// action kind:  + create  ~ update  = skip  ✗ blocked.
func diffToGridData(tmpl domain.Template, diff domain.ReconcileDiff) render.GridData {
	title := fmt.Sprintf("Apply preview: %s", tmpl.Name)
	if tmpl.Description != "" {
		title += " — " + tmpl.Description
	}

	// Index actions and blockers by (rowID, weekday) for fast lookup.
	type cellKey struct {
		rowID   string
		weekday time.Weekday
	}
	actionMap := make(map[cellKey]domain.ActionKind)
	for _, a := range diff.Actions {
		actionMap[cellKey{a.RowID, a.Date.Weekday()}] = a.Kind
	}
	blockerSet := make(map[cellKey]bool)
	for _, b := range diff.Blockers {
		blockerSet[cellKey{b.RowID, b.Date.Weekday()}] = true
	}

	rows := make([]render.GridRow, len(tmpl.Rows))
	for i, r := range tmpl.Rows {
		detail := r.TimeType.Name
		if r.ResolverHints.TargetDisplayName != "" {
			detail += " · " + r.ResolverHints.TargetDisplayName
		}
		var hours [7]float64
		var markers [7]string
		for d := time.Sunday; d <= time.Saturday; d++ {
			h := r.Hours.ForDay(d)
			hours[d] = h
			ck := cellKey{r.ID, d}
			if blockerSet[ck] {
				markers[d] = "✗"
			} else if kind, ok := actionMap[ck]; ok {
				switch kind {
				case domain.ActionCreate:
					markers[d] = "+"
				case domain.ActionUpdate:
					markers[d] = "~"
				case domain.ActionSkip:
					markers[d] = "="
				}
			}
		}
		rows[i] = render.GridRow{
			Label:   r.Label,
			Detail:  detail,
			Ref:     fmt.Sprintf("(%s)", r.Target.Kind),
			Hours:   hours,
			Markers: markers,
		}
	}
	return render.GridData{Title: title, Rows: rows}
}

// dayNameMap maps three-letter lowercase day abbreviations to time.Weekday.
var dayNameMap = map[string]time.Weekday{
	"sun": time.Sunday,
	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
}

// parseDays parses a days filter string. Supports:
//   - Range: "mon-thu" → [Monday, Tuesday, Wednesday, Thursday]
//   - List:  "mon,wed,fri" → [Monday, Wednesday, Friday]
func parseDays(s string) ([]time.Weekday, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	// Range format: "mon-thu"
	if parts := strings.SplitN(s, "-", 2); len(parts) == 2 && !strings.Contains(s, ",") {
		from, ok := dayNameMap[parts[0]]
		if !ok {
			return nil, fmt.Errorf("unknown day %q", parts[0])
		}
		to, ok := dayNameMap[parts[1]]
		if !ok {
			return nil, fmt.Errorf("unknown day %q", parts[1])
		}
		if to < from {
			return nil, fmt.Errorf("invalid range: %s comes before %s", parts[1], parts[0])
		}
		var days []time.Weekday
		for d := from; d <= to; d++ {
			days = append(days, d)
		}
		return days, nil
	}

	// List format: "mon,wed,fri"
	parts := strings.Split(s, ",")
	var days []time.Weekday
	for _, p := range parts {
		p = strings.TrimSpace(p)
		d, ok := dayNameMap[p]
		if !ok {
			return nil, fmt.Errorf("unknown day %q", p)
		}
		days = append(days, d)
	}
	return days, nil
}

// parseOverride parses a single --override value.
// Format: "row-id:day=hours", e.g. "row-01:fri=4".
func parseOverride(s string) (tmplsvc.Override, error) {
	// Split on ":" → rowID, rest
	colonIdx := strings.LastIndex(s, ":")
	if colonIdx < 0 {
		return tmplsvc.Override{}, fmt.Errorf("expected format row-id:day=hours, got %q", s)
	}
	rowID := s[:colonIdx]
	rest := s[colonIdx+1:]

	// Split rest on "=" → day, hours
	eqIdx := strings.Index(rest, "=")
	if eqIdx < 0 {
		return tmplsvc.Override{}, fmt.Errorf("expected format row-id:day=hours, got %q", s)
	}
	dayStr := strings.ToLower(strings.TrimSpace(rest[:eqIdx]))
	hoursStr := strings.TrimSpace(rest[eqIdx+1:])

	day, ok := dayNameMap[dayStr]
	if !ok {
		return tmplsvc.Override{}, fmt.Errorf("unknown day %q in override %q", dayStr, s)
	}
	hours, err := strconv.ParseFloat(hoursStr, 64)
	if err != nil {
		return tmplsvc.Override{}, fmt.Errorf("invalid hours %q in override %q: %w", hoursStr, s, err)
	}
	return tmplsvc.Override{
		RowID: rowID,
		Day:   day,
		Hours: hours,
	}, nil
}
