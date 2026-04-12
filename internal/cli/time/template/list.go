package template

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/render"
	"github.com/ipm/tdx/internal/svc/tmplsvc"
)

func newListCmd() *cobra.Command {
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			store := tmplsvc.NewStore(paths)
			templates, err := store.List()
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})

			if format == render.FormatJSON {
				type jsonRow struct {
					Name        string   `json:"name"`
					Description string   `json:"description,omitempty"`
					Tags        []string `json:"tags,omitempty"`
					Rows        int      `json:"rows"`
					TotalHours  float64  `json:"totalHours"`
				}
				rows := make([]jsonRow, len(templates))
				for i, t := range templates {
					totalHours := 0.0
					for _, r := range t.Rows {
						totalHours += r.Hours.Total()
					}
					rows[i] = jsonRow{
						Name:        t.Name,
						Description: t.Description,
						Tags:        t.Tags,
						Rows:        len(t.Rows),
						TotalHours:  totalHours,
					}
				}
				return render.JSON(w, struct {
					Schema    string    `json:"schema"`
					Templates []jsonRow `json:"templates"`
				}{
					Schema:    "tdx.v1.templateList",
					Templates: rows,
				})
			}

			if len(templates) == 0 {
				_, _ = fmt.Fprintln(w, "no templates saved")
				return nil
			}

			headers := []string{"NAME", "ROWS", "HOURS", "DESCRIPTION"}
			rows := make([][]string, 0, len(templates))
			for _, t := range templates {
				totalHours := 0.0
				for _, r := range t.Rows {
					totalHours += r.Hours.Total()
				}
				rows = append(rows, []string{
					t.Name,
					fmt.Sprintf("%d", len(t.Rows)),
					fmt.Sprintf("%.2f", totalHours),
					t.Description,
				})
			}
			render.Table(w, headers, rows, nil)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	return cmd
}
