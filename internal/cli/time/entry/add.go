package entry

import (
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/render"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/ipm/tdx/internal/svc/timesvc"
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
	cmd.Flags().IntVar(&f.ticket, "ticket", 0, "ticket ID (requires --app)")
	cmd.Flags().IntVar(&f.app, "app", 0, "application ID (required with --ticket)")
	cmd.Flags().IntVar(&f.project, "project", 0, "project ID")
	cmd.Flags().IntVar(&f.plan, "plan", 0, "plan ID (requires --project and --task)")
	cmd.Flags().IntVar(&f.task, "task", 0, "task ID (requires --ticket, or --project with --plan)")
	cmd.Flags().IntVar(&f.issue, "issue", 0, "issue ID (requires --project)")
	cmd.Flags().IntVar(&f.workspace, "workspace", 0, "workspace ID")
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

	if f.typeName == "" {
		return fmt.Errorf("--type is required")
	}

	// Target validation: exactly one of --ticket, --project, --workspace.
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
	if targetCount != 1 {
		return fmt.Errorf("exactly one of --ticket, --project, or --workspace is required")
	}

	// Companion flag validation.
	if f.ticket > 0 && f.app <= 0 {
		return fmt.Errorf("--app is required with --ticket")
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
	tt, ok := domain.FindTimeTypeByName(types, f.typeName)
	if !ok {
		return fmt.Errorf("no time type named %q", f.typeName)
	}

	// ---- 3. Build target from flags ----

	target, projectID := buildTarget(f)

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
		ProjectID:   projectID,
		Description: f.description,
	}

	// ---- 6. Dry run ----

	w := cmd.OutOrStdout()
	if f.dryRun {
		fmt.Fprintf(w, "dry run: would create entry\n")
		fmt.Fprintf(w, "  date:        %s\n", date.Format("2006-01-02"))
		fmt.Fprintf(w, "  minutes:     %d\n", durationMinutes)
		fmt.Fprintf(w, "  hours:       %.2f\n", float64(durationMinutes)/60.0)
		fmt.Fprintf(w, "  type:        %s\n", tt.Name)
		fmt.Fprintf(w, "  target:      %s (kind=%s)\n", targetSummary(target), target.Kind)
		if f.description != "" {
			fmt.Fprintf(w, "  description: %s\n", f.description)
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

	fmt.Fprintf(w, "created entry %d\n", entry.ID)
	printEntry(w, entry)
	return nil
}

// buildTarget translates CLI flags into a domain.Target and the optional
// wire ProjectID (used only for projectTask / projectIssue).
func buildTarget(f addFlags) (domain.Target, int) {
	switch {
	case f.ticket > 0 && f.task > 0:
		return domain.Target{
			Kind:   domain.TargetTicketTask,
			AppID:  f.app,
			ItemID: f.ticket,
			TaskID: f.task,
		}, 0

	case f.ticket > 0:
		return domain.Target{
			Kind:   domain.TargetTicket,
			AppID:  f.app,
			ItemID: f.ticket,
		}, 0

	case f.project > 0 && f.plan > 0 && f.task > 0:
		return domain.Target{
			Kind:   domain.TargetProjectTask,
			ItemID: f.plan,
			TaskID: f.task,
		}, f.project

	case f.project > 0 && f.issue > 0:
		return domain.Target{
			Kind:   domain.TargetProjectIssue,
			ItemID: f.issue,
		}, f.project

	case f.project > 0:
		return domain.Target{
			Kind:   domain.TargetProject,
			ItemID: f.project,
		}, 0

	case f.workspace > 0:
		return domain.Target{
			Kind:   domain.TargetWorkspace,
			ItemID: f.workspace,
		}, 0

	default:
		// Should be unreachable due to earlier validation.
		return domain.Target{}, 0
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
	default:
		return fmt.Sprintf("item %d", t.ItemID)
	}
}
