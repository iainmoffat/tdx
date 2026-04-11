package week

import (
	"fmt"
	"time"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/render"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/ipm/tdx/internal/svc/timesvc"
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

			report, err := tsvc.GetWeekReport(cmd.Context(), profileName, day)
			if err != nil {
				return err
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
	return cmd
}
