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

type lockedDaysJSON struct {
	Schema string             `json:"schema"`
	From   string             `json:"from"`
	To     string             `json:"to"`
	Days   []domain.LockedDay `json:"days"`
}

func newLockedCmd() *cobra.Command {
	var (
		profileFlag string
		fromFlag    string
		toFlag      string
		jsonFlag    bool
	)

	cmd := &cobra.Command{
		Use:   "locked",
		Short: "List locked days in a date range (defaults to the current week)",
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

			var from, to time.Time
			if fromFlag != "" || toFlag != "" {
				if fromFlag == "" || toFlag == "" {
					return fmt.Errorf("--from and --to must be given together")
				}
				var err error
				from, err = time.ParseInLocation("2006-01-02", fromFlag, domain.EasternTZ)
				if err != nil {
					return fmt.Errorf("invalid --from: %w", err)
				}
				to, err = time.ParseInLocation("2006-01-02", toFlag, domain.EasternTZ)
				if err != nil {
					return fmt.Errorf("invalid --to: %w", err)
				}
			} else {
				w := domain.WeekRefContaining(time.Now())
				from = w.StartDate
				to = w.EndDate
			}

			days, err := tsvc.GetLockedDays(cmd.Context(), profileName, from, to)
			if err != nil {
				return err
			}

			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})
			if format == render.FormatJSON {
				return render.JSON(cmd.OutOrStdout(), lockedDaysJSON{
					Schema: "tdx.v1.lockedDays",
					From:   from.Format("2006-01-02"),
					To:     to.Format("2006-01-02"),
					Days:   days,
				})
			}

			w := cmd.OutOrStdout()
			if len(days) == 0 {
				_, _ = fmt.Fprintln(w, "no locked days in range")
				return nil
			}
			for _, d := range days {
				if d.Reason != "" {
					_, _ = fmt.Fprintf(w, "%s  %s\n", d.Date.Format("2006-01-02"), d.Reason)
				} else {
					_, _ = fmt.Fprintln(w, d.Date.Format("2006-01-02"))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to active profile)")
	cmd.Flags().StringVar(&fromFlag, "from", "", "range start YYYY-MM-DD (defaults to current week)")
	cmd.Flags().StringVar(&toFlag, "to", "", "range end YYYY-MM-DD (defaults to current week)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	return cmd
}
