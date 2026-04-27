package template

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
)

func newShowCmd() *cobra.Command {
	var (
		jsonFlag    bool
		profileFlag string
	)

	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show a saved template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			store := tmplsvc.NewStore(paths)
			tmpl, err := store.Load(profile, args[0])
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})

			if format == render.FormatJSON {
				return render.JSON(w, struct {
					Schema   string          `json:"schema"`
					Template domain.Template `json:"template"`
				}{
					Schema:   "tdx.v1.template",
					Template: tmpl,
				})
			}

			render.Grid(w, templateToGridData(tmpl))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

// templateToGridData converts a domain.Template into render.GridData
// for the grid renderer.
func templateToGridData(tmpl domain.Template) render.GridData {
	title := tmpl.Name
	if tmpl.Description != "" {
		title += " — " + tmpl.Description
	}
	var subtitle string
	if tmpl.DerivedFrom != nil {
		subtitle = fmt.Sprintf("(derived from %s)", tmpl.DerivedFrom.WeekStart.Format("2006-01-02"))
	}
	rows := make([]render.GridRow, len(tmpl.Rows))
	for i, r := range tmpl.Rows {
		detail := r.TimeType.Name
		if r.ResolverHints.TargetDisplayName != "" {
			detail += " · " + r.ResolverHints.TargetDisplayName
		}
		rows[i] = render.GridRow{
			Label:  r.Label,
			Detail: detail,
			Group:  r.Target.GroupName,
			Ref:    fmt.Sprintf("(%s)", r.Target.Kind),
			Hours: [7]float64{
				r.Hours.Sun, r.Hours.Mon, r.Hours.Tue, r.Hours.Wed,
				r.Hours.Thu, r.Hours.Fri, r.Hours.Sat,
			},
		}
	}
	return render.GridData{Title: title, Subtitle: subtitle, Rows: rows}
}
