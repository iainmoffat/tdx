package week

import (
	"fmt"
	"io"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
	"github.com/spf13/cobra"
)

type weekReportJSON struct {
	Schema       string              `json:"schema"`
	WeekRef      domain.WeekRef      `json:"weekRef"`
	UserUID      string              `json:"userUID,omitempty"`
	TotalHours   float64             `json:"totalHours"`
	TotalMinutes int                 `json:"totalMinutes"`
	Status       domain.ReportStatus `json:"status"`
	Days         []domain.DaySummary `json:"days"`
	Entries      []domain.TimeEntry  `json:"entries"`
}

func newShowCmd() *cobra.Command {
	var (
		profileFlag string
		jsonFlag    bool
		draftFlag   bool
		nameFlag    string
	)

	cmd := &cobra.Command{
		Use:   "show [date]",
		Short: "Show the week containing the given date (defaults to today)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			tsvc := timesvc.New(paths)
			drafts := draftsvc.NewService(paths, tsvc)

			profileName, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}

			day := time.Now().In(domain.EasternTZ)
			if len(args) == 1 {
				parsed, err := time.ParseInLocation("2006-01-02", args[0], domain.EasternTZ)
				if err != nil {
					return fmt.Errorf("invalid date %q: %w", args[0], err)
				}
				day = parsed
			}
			weekStart := domain.WeekRefContaining(day).StartDate

			// Draft mode
			if draftFlag {
				draft, err := drafts.Store().Load(profileName, weekStart, nameFlag)
				if err != nil {
					return err
				}
				w := cmd.OutOrStdout()
				if jsonFlag {
					return render.JSON(w, struct {
						Schema string           `json:"schema"`
						Draft  domain.WeekDraft `json:"draft"`
					}{Schema: "tdx.v1.weekDraft", Draft: draft})
				}
				renderDraftAsWeekReport(w, draft)
				return nil
			}

			// Live mode (existing behavior)
			report, err := tsvc.GetWeekReport(cmd.Context(), profileName, day)
			if err != nil {
				return err
			}

			// Banner: surface a default draft if one exists for this week.
			if drafts.Store().Exists(profileName, weekStart, "default") {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"Local draft exists for this week. View with `tdx time week show %s --draft`.\n\n",
					weekStart.Format("2006-01-02"))
			}

			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})
			if format == render.FormatJSON {
				return render.JSON(cmd.OutOrStdout(), weekReportJSON{
					Schema:       "tdx.v1.weekReport",
					WeekRef:      report.WeekRef,
					UserUID:      report.UserUID,
					TotalHours:   report.TotalHours(),
					TotalMinutes: report.TotalMinutes,
					Status:       report.Status,
					Days:         report.Days,
					Entries:      report.Entries,
				})
			}

			render.WeekGrid(cmd.OutOrStdout(), report)
			return nil
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to active profile)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().BoolVar(&draftFlag, "draft", false, "render the local draft instead of the live week")
	cmd.Flags().StringVar(&nameFlag, "name", "default", "draft name (only with --draft)")
	return cmd
}

// renderDraftAsWeekReport renders a draft using the existing live-week grid
// renderer by synthesizing a WeekReport. Cell-state annotations are not shown
// in this view; preview/edit views handle that.
func renderDraftAsWeekReport(w io.Writer, d domain.WeekDraft) {
	weekStart := d.WeekStart
	weekEnd := weekStart.AddDate(0, 0, 6)
	var entries []domain.TimeEntry
	for _, row := range d.Rows {
		for _, cell := range row.Cells {
			if cell.Hours == 0 {
				continue
			}
			entries = append(entries, domain.TimeEntry{
				ID:          cell.SourceEntryID,
				Date:        weekStart.AddDate(0, 0, int(cell.Day)),
				Minutes:     int(cell.Hours * 60),
				Target:      row.Target,
				TimeType:    row.TimeType,
				Billable:    row.Billable,
				Description: row.Description,
			})
		}
	}
	days := make([]domain.DaySummary, 7)
	for i := range days {
		date := weekStart.AddDate(0, 0, i)
		days[i] = domain.DaySummary{Date: date}
	}
	var totalMin int
	for _, e := range entries {
		totalMin += e.Minutes
		// Determine which day of week this entry belongs to.
		dayIdx := int(e.Date.Weekday())
		days[dayIdx].Minutes += e.Minutes
	}
	report := domain.WeekReport{
		WeekRef:      domain.WeekRef{StartDate: weekStart, EndDate: weekEnd},
		TotalMinutes: totalMin,
		Status:       d.Provenance.RemoteStatus,
		Days:         days,
		Entries:      entries,
	}
	_, _ = fmt.Fprintf(w, "Draft: %s/%s\n\n", weekStart.Format("2006-01-02"), d.Name)
	render.WeekGrid(w, report)
}
