package entry

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

type updateFlags struct {
	profile     string
	date        string
	hours       float64
	minutes     int
	typeName    string
	description string
	dryRun      bool
	json        bool
}

type entryUpdateJSON struct {
	Schema string           `json:"schema"`
	Entry  domain.TimeEntry `json:"entry"`
}

func newUpdateCmd() *cobra.Command {
	var f updateFlags

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an existing time entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid entry id %q: %w", args[0], err)
			}
			if id <= 0 {
				return fmt.Errorf("entry id must be a positive integer, got %d", id)
			}
			return runUpdate(cmd, id, f)
		},
	}

	cmd.Flags().StringVar(&f.profile, "profile", "", "profile name (defaults to active profile)")
	cmd.Flags().StringVar(&f.date, "date", "", "new entry date (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&f.hours, "hours", 0, "new duration in hours (mutually exclusive with --minutes)")
	cmd.Flags().IntVar(&f.minutes, "minutes", 0, "new duration in minutes (mutually exclusive with --hours)")
	cmd.Flags().StringVar(&f.typeName, "type", "", "new time type name (case-insensitive)")
	cmd.Flags().StringVarP(&f.description, "description", "d", "", "new description of work performed")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "preview changes without applying them")
	cmd.Flags().BoolVar(&f.json, "json", false, "emit JSON output")

	return cmd
}

func runUpdate(cmd *cobra.Command, id int, f updateFlags) error {
	// ---- 1. Validate flag mutual exclusion ----

	hoursSet := cmd.Flags().Changed("hours")
	minutesSet := cmd.Flags().Changed("minutes")
	if hoursSet && minutesSet {
		return fmt.Errorf("--hours and --minutes are mutually exclusive")
	}

	// ---- 2. Build EntryUpdate from provided flags ----

	var update domain.EntryUpdate

	if cmd.Flags().Changed("date") {
		parsed, err := time.ParseInLocation("2006-01-02", f.date, domain.EasternTZ)
		if err != nil {
			return fmt.Errorf("invalid --date: %w", err)
		}
		update.Date = &parsed
	}

	if hoursSet {
		mins := int(math.Round(f.hours * 60))
		update.Minutes = &mins
	} else if minutesSet {
		update.Minutes = &f.minutes
	}

	if cmd.Flags().Changed("description") {
		update.Description = &f.description
	}

	// Check "nothing to update" before network calls.
	// --type is handled after profile resolution, so check it here via the flag.
	typeSet := cmd.Flags().Changed("type")
	if update.IsEmpty() && !typeSet {
		return fmt.Errorf("nothing to update: provide at least one of --date, --hours, --minutes, --type, or --description")
	}

	// ---- 3. Resolve profile ----

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

	// ---- 4. Resolve --type if set ----

	if typeSet {
		types, err := tsvc.ListTimeTypes(cmd.Context(), profileName)
		if err != nil {
			return fmt.Errorf("lookup time types: %w", err)
		}
		tt, ok := domain.FindTimeTypeByName(types, f.typeName)
		if !ok {
			return fmt.Errorf("no time type named %q", f.typeName)
		}
		update.TimeTypeID = &tt.ID
	}

	// ---- 5. Fetch existing entry (needed for validation date + dry-run diff) ----

	existing, err := tsvc.GetEntry(cmd.Context(), profileName, id)
	if err != nil {
		return err
	}

	// ---- 6. Pre-write validation ----
	// When changing the date, validate both the old date and the new date.

	datesToCheck := []time.Time{existing.Date}
	if update.Date != nil {
		datesToCheck = append(datesToCheck, *update.Date)
	}

	for _, checkDate := range datesToCheck {
		locked, err := tsvc.GetLockedDays(cmd.Context(), profileName, checkDate, checkDate)
		if err != nil {
			return err
		}
		for _, ld := range locked {
			ly, lm, lday := ld.Date.Date()
			dy, dm, dday := checkDate.Date()
			if ly == dy && lm == dm && lday == dday {
				return fmt.Errorf("%w: %s", domain.ErrDayLocked, checkDate.Format("2006-01-02"))
			}
		}

		report, err := tsvc.GetWeekReport(cmd.Context(), profileName, checkDate)
		if err != nil {
			return err
		}
		if report.Status != domain.ReportOpen {
			return fmt.Errorf("%w: status is %s", domain.ErrWeekSubmitted, report.Status)
		}
	}

	w := cmd.OutOrStdout()

	// ---- 7. Dry run ----

	if f.dryRun {
		_, _ = fmt.Fprintf(w, "dry run: would update entry %d\n", id)
		if update.Description != nil {
			_, _ = fmt.Fprintf(w, "  description: %q → %q\n", existing.Description, *update.Description)
		}
		if update.Date != nil {
			_, _ = fmt.Fprintf(w, "  date: %s → %s\n", existing.Date.Format("2006-01-02"), update.Date.Format("2006-01-02"))
		}
		if update.Minutes != nil {
			_, _ = fmt.Fprintf(w, "  minutes: %d → %d\n", existing.Minutes, *update.Minutes)
		}
		if update.TimeTypeID != nil {
			_, _ = fmt.Fprintf(w, "  type id: %d → %d\n", existing.TimeType.ID, *update.TimeTypeID)
		}
		return nil
	}

	// ---- 8. Apply update ----

	entry, err := tsvc.UpdateEntry(cmd.Context(), profileName, id, update)
	if err != nil {
		return err
	}

	// ---- 9. Output ----

	format := render.ResolveFormat(render.Flags{JSON: f.json})
	if format == render.FormatJSON {
		return render.JSON(w, entryUpdateJSON{
			Schema: "tdx.v1.entry",
			Entry:  entry,
		})
	}

	_, _ = fmt.Fprintf(w, "updated entry %d\n", entry.ID)
	printEntry(w, entry)
	return nil
}
