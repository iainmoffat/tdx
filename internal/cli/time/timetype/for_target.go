package timetype

import (
	"fmt"
	"strconv"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/render"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/ipm/tdx/internal/svc/timesvc"
	"github.com/spf13/cobra"
)

type typeForJSON struct {
	Schema string            `json:"schema"`
	Target domain.Target     `json:"target"`
	Types  []domain.TimeType `json:"types"`
}

func newForCmd() *cobra.Command {
	var (
		profileFlag string
		appFlag     int
		taskFlag    int
		jsonFlag    bool
	)

	cmd := &cobra.Command{
		Use:   "for <kind> <id>",
		Short: "Show time types valid for a specific work item",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := domain.TargetKind(args[0])
			if !kind.IsKnown() {
				return fmt.Errorf("unknown kind %q: supported kinds are ticket, ticketTask, project, projectIssue, workspace, timeoff, request", args[0])
			}
			if !kind.SupportsComponentLookup() {
				return fmt.Errorf("kind %q does not support component lookup", args[0])
			}

			id, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid id %q: %w", args[1], err)
			}

			// Only ticket and ticketTask use AppID in TD's URL paths; the other
			// kinds (project, projectIssue, workspace, timeoff, request) hit
			// /component/<kind>/<id> and do not need an application context.
			if (kind == domain.TargetTicket || kind == domain.TargetTicketTask) && appFlag <= 0 {
				return fmt.Errorf("--app is required for kind %q", kind)
			}
			// Task-bearing kinds require --task. ProjectIssue uses --task as the
			// issue ID slot (see timesvc.componentPathFor). TargetProjectTask is
			// excluded by SupportsComponentLookup above; if that ever changes,
			// re-add it here.
			if (kind == domain.TargetTicketTask || kind == domain.TargetProjectIssue) && taskFlag <= 0 {
				return fmt.Errorf("kind %q requires --task", kind)
			}

			target := domain.Target{
				Kind:   kind,
				AppID:  appFlag,
				ItemID: id,
				TaskID: taskFlag,
			}

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

			types, err := tsvc.TimeTypesForTarget(cmd.Context(), profileName, target)
			if err != nil {
				return err
			}

			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})
			if format == render.FormatJSON {
				return render.JSON(cmd.OutOrStdout(), typeForJSON{
					Schema: "tdx.v1.timeTypesForTarget",
					Target: target,
					Types:  types,
				})
			}

			headers := []string{"ID", "NAME", "BILLABLE", "LIMITED"}
			rows := make([][]string, 0, len(types))
			for _, tt := range types {
				rows = append(rows, []string{
					fmt.Sprintf("%d", tt.ID),
					tt.Name,
					fmt.Sprintf("%t", tt.Billable),
					fmt.Sprintf("%t", tt.Limited),
				})
			}
			render.Table(cmd.OutOrStdout(), headers, rows, nil)
			return nil
		},
	}

	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to active profile)")
	cmd.Flags().IntVar(&appFlag, "app", 0, "application ID (required)")
	cmd.Flags().IntVar(&taskFlag, "task", 0, "task or issue ID (required for ticketTask and projectIssue)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	return cmd
}
