package timetype

import (
	"fmt"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/render"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/ipm/tdx/internal/svc/timesvc"
	"github.com/spf13/cobra"
)

type typeListJSON struct {
	Schema string            `json:"schema"`
	Types  []domain.TimeType `json:"types"`
}

func newListCmd() *cobra.Command {
	var (
		profileFlag string
		jsonFlag    bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all visible time types",
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

			types, err := tsvc.ListTimeTypes(cmd.Context(), profileName)
			if err != nil {
				return err
			}

			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})
			if format == render.FormatJSON {
				return render.JSON(cmd.OutOrStdout(), typeListJSON{
					Schema: "tdx.v1.timeTypes",
					Types:  types,
				})
			}

			headers := []string{"ID", "NAME", "BILLABLE", "LIMITED", "DESCRIPTION"}
			rows := make([][]string, 0, len(types))
			for _, tt := range types {
				rows = append(rows, []string{
					fmt.Sprintf("%d", tt.ID),
					tt.Name,
					fmt.Sprintf("%t", tt.Billable),
					fmt.Sprintf("%t", tt.Limited),
					tt.Description,
				})
			}
			render.Table(cmd.OutOrStdout(), headers, rows, nil)
			return nil
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to active profile)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	return cmd
}
