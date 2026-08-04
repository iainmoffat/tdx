package entry

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type addFlags struct {
	profile     string
	date        string
	hours       float64
	minutes     int
	typeName    string
	description string
	ticket      int
	app         int
	project     int
	plan        int
	task        int
	issue       int
	workspace   int
	timeOff     bool
	timeOffID   int
	dryRun      bool
	json        bool
}

type entryAddJSON struct {
	Schema string           `json:"schema"`
	Entry  domain.TimeEntry `json:"entry"`
}

func newAddCmd() *cobra.Command {
	var f addFlags

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new time entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, f)
		},
	}

	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name (defaults to active profile)")
	cmd.Flags().StringVar(&f.date, "date", "", "entry date (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&f.hours, "hours", 0, "duration in hours (mutually exclusive with --minutes)")
	cmd.Flags().IntVar(&f.minutes, "minutes", 0, "duration in minutes (mutually exclusive with --hours)")
	cmd.Flags().StringVar(&f.typeName, "type", "", "time type name (case-insensitive)")
	cmd.Flags().StringVarP(&f.description, "description", "d", "", "description of work performed")
	cmd.Flags().IntVar(&f.ticket, "ticket", 0, "ticket ID (uses profile's default app if --app not set)")
	cmd.Flags().IntVar(&f.app, "app", 0, "application ID (overrides profile default with --ticket; required with --workspace)")
	cmd.Flags().IntVar(&f.project, "project", 0, "project ID")
	cmd.Flags().IntVar(&f.plan, "plan", 0, "plan ID (requires --project and --task)")
	cmd.Flags().IntVar(&f.task, "task", 0, "task ID (requires --ticket, or --project with --plan)")
	cmd.Flags().IntVar(&f.issue, "issue", 0, "issue ID (requires --project)")
	cmd.Flags().IntVar(&f.workspace, "workspace", 0, "workspace ID")
	cmd.Flags().BoolVar(&f.timeOff, "time-off", false, "log time off / leave (time-off ID is auto-discovered from your recent leave entries)")
	cmd.Flags().IntVar(&f.timeOffID, "time-off-id", 0, "override the time-off item ID (requires --time-off)")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "preview without creating the entry")
	cmd.Flags().BoolVar(&f.json, "json", false, "emit JSON output")

	return cmd
}

func runAdd(cmd *cobra.Command, f addFlags) error {
	// ---- 1. Validate flags ----

	if f.date == "" {
		return fmt.Errorf("--date is required")
	}
	date, err := time.ParseInLocation("2006-01-02", f.date, domain.EasternTZ)
	if err != nil {
		return fmt.Errorf("invalid --date: %w", err)
	}

	hoursSet := cmd.Flags().Changed("hours")
	minutesSet := cmd.Flags().Changed("minutes")
	if hoursSet == minutesSet {
		// Both set or both unset.
		return fmt.Errorf("exactly one of --hours or --minutes is required")
	}

	var durationMinutes int
	if hoursSet {
		durationMinutes = int(math.Round(f.hours * 60))
	} else {
		durationMinutes = f.minutes
	}
	if durationMinutes <= 0 {
		return fmt.Errorf("duration must be positive")
	}

	// --type may be omitted for --time-off: the tenant's single time-off type
	// is used. Every other target still requires it.
	if f.typeName == "" && !f.timeOff {
		return fmt.Errorf("--type is required")
	}

	// Target validation: exactly one of --ticket, --project, --workspace, --time-off.
	targetCount := 0
	if f.ticket > 0 {
		targetCount++
	}
	if f.project > 0 {
		targetCount++
	}
	if f.workspace > 0 {
		targetCount++
	}
	if f.timeOff {
		targetCount++
	}
	if targetCount != 1 {
		if !f.timeOff {
			return fmt.Errorf("exactly one of --ticket, --project, or --workspace is required")
		}
		return fmt.Errorf("exactly one of --ticket, --project, --workspace, or --time-off is required")
	}

	if f.timeOffID > 0 && !f.timeOff {
		return fmt.Errorf("--time-off-id requires --time-off")
	}
	if f.timeOff && (f.app > 0 || f.plan > 0 || f.task > 0 || f.issue > 0) {
		return fmt.Errorf("--time-off cannot be combined with --app, --plan, --task, or --issue")
	}

	// Companion flag validation.
	if f.ticket > 0 && f.app <= 0 {
		// Fall back to the profile's default TicketAppID (set via
		// `tdx ticket app use <id>`) before erroring out.
		profileDefault := 0
		if paths, perr := config.ResolvePaths(); perr == nil {
			auth := authsvc.New(paths)
			if pname, rerr := auth.ResolveProfile(f.profile); rerr == nil {
				if prof, gerr := config.NewProfileStore(paths).GetProfile(pname); gerr == nil {
					profileDefault = prof.TicketAppID
				}
			}
		}
		f.app = resolveTicketAppID(f.app, profileDefault)
		if f.app <= 0 {
			return fmt.Errorf("--app is required with --ticket (or run `tdx ticket app use <id>` to set a profile default)")
		}
	}
	if f.plan > 0 && (f.project <= 0 || f.task <= 0) {
		return fmt.Errorf("--plan requires both --project and --task")
	}
	if f.task > 0 && f.project > 0 && f.plan <= 0 {
		return fmt.Errorf("--task with --project requires --plan")
	}

	// ---- 2. Resolve profile, user, time type ----

	paths, err := config.ResolvePaths()
	if err != nil {
		return err
	}
	auth := authsvc.New(paths)
	tsvc := timesvc.New(paths)

	profileName, err := auth.ResolveProfile(f.profile)
	if err != nil {
		return err
	}

	user, err := auth.WhoAmI(cmd.Context(), profileName)
	if err != nil {
		return fmt.Errorf("could not resolve current user: %w", err)
	}

	types, err := tsvc.ListTimeTypes(cmd.Context(), profileName)
	if err != nil {
		return fmt.Errorf("lookup time types: %w", err)
	}
	var tt domain.TimeType
	if f.typeName != "" {
		var ok bool
		tt, ok = domain.FindTimeTypeByName(types, f.typeName)
		if !ok {
			return fmt.Errorf("no time type named %q", f.typeName)
		}
		if f.timeOff && !tt.IsTimeOff {
			return fmt.Errorf("time type %q is not a time-off type", tt.Name)
		}
	} else {
		// Only reachable with --time-off (validated above).
		var derr error
		tt, derr = domain.DefaultTimeOffType(types)
		if derr != nil {
			return fmt.Errorf("--type is required (could not pick a default time-off type): %w", derr)
		}
	}

	if f.timeOff {
		itemID, rerr := tsvc.ResolveTimeOffItemID(cmd.Context(), profileName, user.UID, f.timeOffID)
		if rerr != nil {
			if errors.Is(rerr, domain.ErrTimeOffIDUnknown) {
				return fmt.Errorf("couldn't determine your time-off ID — log one leave entry in the TD web UI first, or pass --time-off-id N")
			}
			return rerr
		}
		f.timeOffID = itemID
	}

	// ---- 3. Build target from flags ----

	target := buildTarget(f)

	// ---- 4. Pre-write validation ----

	locked, err := tsvc.GetLockedDays(cmd.Context(), profileName, date, date)
	if err != nil {
		return err
	}
	for _, ld := range locked {
		ly, lm, lday := ld.Date.Date()
		dy, dm, dday := date.Date()
		if ly == dy && lm == dm && lday == dday {
			return fmt.Errorf("%w: %s", domain.ErrDayLocked, date.Format("2006-01-02"))
		}
	}

	report, err := tsvc.GetWeekReport(cmd.Context(), profileName, date)
	if err != nil {
		return err
	}
	if report.Status != domain.ReportOpen {
		return fmt.Errorf("%w: status is %s", domain.ErrWeekSubmitted, report.Status)
	}

	// ---- 5. Build EntryInput ----

	input := domain.EntryInput{
		UserUID:     user.UID,
		Date:        date,
		Minutes:     durationMinutes,
		TimeTypeID:  tt.ID,
		Billable:    tt.Billable,
		Target:      target,
		Description: f.description,
	}

	// ---- 6. Dry run ----

	w := cmd.OutOrStdout()
	if f.dryRun {
		_, _ = fmt.Fprintf(w, "dry run: would create entry\n")
		_, _ = fmt.Fprintf(w, "  date:        %s\n", date.Format("2006-01-02"))
		_, _ = fmt.Fprintf(w, "  minutes:     %d\n", durationMinutes)
		_, _ = fmt.Fprintf(w, "  hours:       %.2f\n", float64(durationMinutes)/60.0)
		_, _ = fmt.Fprintf(w, "  type:        %s\n", tt.Name)
		_, _ = fmt.Fprintf(w, "  target:      %s (kind=%s)\n", targetSummary(target), target.Kind)
		if f.description != "" {
			_, _ = fmt.Fprintf(w, "  description: %s\n", f.description)
		}
		return nil
	}

	// ---- 7. Create entry ----

	entry, err := tsvc.AddEntry(cmd.Context(), profileName, input)
	if err != nil {
		return err
	}

	// ---- 8. Output ----

	format := render.ResolveFormat(render.Flags{JSON: f.json})
	if format == render.FormatJSON {
		return render.JSON(w, entryAddJSON{
			Schema: "tdx.v1.entryAdd",
			Entry:  entry,
		})
	}

	_, _ = fmt.Fprintf(w, "created entry %d\n", entry.ID)
	printEntry(w, entry)
	return nil
}

// buildTarget translates CLI flags into a domain.Target and the optional
// wire ProjectID (used only for projectTask / projectIssue).
func buildTarget(f addFlags) domain.Target {
	switch {
	case f.timeOff:
		return domain.Target{
			Kind:   domain.TargetTimeOff,
			ItemID: f.timeOffID,
		}

	case f.ticket > 0 && f.task > 0:
		return domain.Target{
			Kind:   domain.TargetTicketTask,
			AppID:  f.app,
			ItemID: f.ticket,
			TaskID: f.task,
		}

	case f.ticket > 0:
		return domain.Target{
			Kind:   domain.TargetTicket,
			AppID:  f.app,
			ItemID: f.ticket,
		}

	case f.project > 0 && f.plan > 0 && f.task > 0:
		return domain.Target{
			Kind:      domain.TargetProjectTask,
			ItemID:    f.plan,
			TaskID:    f.task,
			ProjectID: f.project,
		}

	case f.project > 0 && f.issue > 0:
		return domain.Target{
			Kind:      domain.TargetProjectIssue,
			ItemID:    f.issue,
			ProjectID: f.project,
		}

	case f.project > 0:
		return domain.Target{
			Kind:   domain.TargetProject,
			ItemID: f.project,
		}

	case f.workspace > 0:
		return domain.Target{
			Kind:   domain.TargetWorkspace,
			ItemID: f.workspace,
		}

	default:
		// Should be unreachable due to earlier validation.
		return domain.Target{}
	}
}

// targetSummary renders a short human-readable description for dry-run output.
func targetSummary(t domain.Target) string {
	switch t.Kind {
	case domain.TargetTicket:
		return fmt.Sprintf("ticket %d (app %d)", t.ItemID, t.AppID)
	case domain.TargetTicketTask:
		return fmt.Sprintf("ticket %d task %d (app %d)", t.ItemID, t.TaskID, t.AppID)
	case domain.TargetProject:
		return fmt.Sprintf("project %d", t.ItemID)
	case domain.TargetProjectTask:
		return fmt.Sprintf("plan %d task %d", t.ItemID, t.TaskID)
	case domain.TargetProjectIssue:
		return fmt.Sprintf("issue %d", t.ItemID)
	case domain.TargetWorkspace:
		return fmt.Sprintf("workspace %d", t.ItemID)
	case domain.TargetTimeOff:
		return fmt.Sprintf("time-off (id %d)", t.ItemID)
	default:
		return fmt.Sprintf("item %d", t.ItemID)
	}
}

// resolveTicketAppID returns the appID to use for a --ticket invocation:
//   - if explicit > 0, use that (caller-provided --app)
//   - else if profileTicketAppID > 0, use the profile default
//   - else 0 (caller treats as "no appID resolved" and errors out)
//
// Pure helper for testability. The cobra glue reads the profile from disk
// and passes prof.TicketAppID as the second argument; profile-load failures
// surface as profileTicketAppID=0 (no propagation), matching the silent-
// fallback pattern used elsewhere in the ticket commands.
func resolveTicketAppID(explicit, profileTicketAppID int) int {
	if explicit > 0 {
		return explicit
	}
	if profileTicketAppID > 0 {
		return profileTicketAppID
	}
	return 0
}
