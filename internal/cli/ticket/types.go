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

func newTypesCmd(svc ticketsvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "types",
		Short: "Inspect ticket types in the current app",
	}
	cmd.AddCommand(newTypesListCmd(svc))
	return cmd
}

func newTypesListCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		appID       int
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ticket types",
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
			return runTypesList(cmd.Context(), cmd.OutOrStdout(), s, profile, appID, jsonFlag)
		},
	}
	cmd.Flags().IntVar(&appID, "app", 0, "ticket app id (overrides profile default)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runTypesList(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, appID int, jsonOut bool) error {
	types, err := svc.ListTypes(ctx, profile, appID)
	if err != nil {
		return err
	}
	if jsonOut {
		type typeJSON struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Active      bool   `json:"active"`
		}
		out := make([]typeJSON, 0, len(types))
		for _, t := range types {
			out = append(out, typeJSON{ID: t.ID, Name: t.Name, Description: t.Description, Active: t.Active})
		}
		return render.JSON(w, struct {
			Schema string     `json:"schema"`
			Types  []typeJSON `json:"types"`
		}{Schema: "tdx.v1.ticketTypeList", Types: out})
	}
	if len(types) == 0 {
		_, _ = fmt.Fprintln(w, "no ticket types found")
		return nil
	}
	headers := []string{"ID", "NAME", "DESCRIPTION", "ACTIVE"}
	rows := make([][]string, 0, len(types))
	for _, t := range types {
		active := "yes"
		if !t.Active {
			active = "no"
		}
		rows = append(rows, []string{strconv.Itoa(t.ID), t.Name, t.Description, active})
	}
	render.Table(w, headers, rows, nil)
	return nil
}
