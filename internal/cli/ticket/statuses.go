package ticket

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

func newStatusesCmd(svc ticketsvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "statuses",
		Short: "Inspect ticket statuses in the current app",
	}
	cmd.AddCommand(newStatusesListCmd(svc))
	return cmd
}

func newStatusesListCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ticket statuses",
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
			s := svc
			if s == nil {
				s = ticketsvc.New(paths)
			}
			return runStatusesList(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runStatusesList(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID int, jsonOut bool) error {
	statuses, err := svc.ListStatuses(ctx, profile, appID)
	if err != nil {
		return err
	}
	if jsonOut {
		type statusJSON struct {
			ID        int     `json:"id"`
			Name      string  `json:"name"`
			IsClosed  bool    `json:"isClosed"`
			IsDefault bool    `json:"isDefault"`
			Order     float64 `json:"order"`
		}
		out := make([]statusJSON, 0, len(statuses))
		for _, s := range statuses {
			out = append(out, statusJSON{ID: s.ID, Name: s.Name, IsClosed: s.IsClosed, IsDefault: s.IsDefault, Order: s.Order})
		}
		return render.JSON(w, struct {
			Schema   string       `json:"schema"`
			Statuses []statusJSON `json:"statuses"`
		}{Schema: "tdx.v1.ticketStatusList", Statuses: out})
	}
	if len(statuses) == 0 {
		_, _ = fmt.Fprintln(w, "no ticket statuses found")
		return nil
	}
	headers := []string{"ID", "NAME", "IS-CLOSED", "IS-DEFAULT", "ORDER"}
	rows := make([][]string, 0, len(statuses))
	for _, s := range statuses {
		closed := "no"
		if s.IsClosed {
			closed = "yes"
		}
		def := "no"
		if s.IsDefault {
			def = "yes"
		}
		rows = append(rows, []string{
			strconv.Itoa(s.ID),
			s.Name,
			closed,
			def,
			strconv.FormatFloat(s.Order, 'f', -1, 64),
		})
	}
	render.Table(w, headers, rows, nil)
	return nil
}
