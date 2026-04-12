package template

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/render"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/ipm/tdx/internal/svc/timesvc"
	"github.com/ipm/tdx/internal/svc/tmplsvc"
)

func newDeriveCmd() *cobra.Command {
	var (
		fromWeek    string
		description string
		profileFlag string
		jsonFlag    bool
	)

	cmd := &cobra.Command{
		Use:   "derive <name>",
		Short: "Create a template from a live week's entries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if fromWeek == "" {
				return fmt.Errorf("--from-week is required")
			}
			weekDate, err := time.ParseInLocation("2006-01-02", fromWeek, domain.EasternTZ)
			if err != nil {
				return fmt.Errorf("invalid --from-week: %w", err)
			}

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

			tmpl, err := svc.Derive(cmd.Context(), profileName, name, weekDate)
			if err != nil {
				return err
			}

			if description != "" {
				tmpl.Description = description
				if err := svc.Store().Save(tmpl); err != nil {
					return err
				}
			}

			w := cmd.OutOrStdout()
			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})
			if format == render.FormatJSON {
				return render.JSON(w, struct {
					Schema   string          `json:"schema"`
					Template domain.Template `json:"template"`
				}{
					Schema:   "tdx.v1.templateDerive",
					Template: tmpl,
				})
			}

			totalHours := 0.0
			for _, r := range tmpl.Rows {
				totalHours += r.Hours.Total()
			}
			_, _ = fmt.Fprintf(w, "derived template %q from week of %s (%d rows, %.1f hours)\n",
				tmpl.Name, tmpl.DerivedFrom.WeekStart.Format("2006-01-02"),
				len(tmpl.Rows), totalHours)
			return nil
		},
	}

	cmd.Flags().StringVar(&fromWeek, "from-week", "", "source week date (YYYY-MM-DD)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "template description")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	return cmd
}
