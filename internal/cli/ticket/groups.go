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

func newGroupsCmd(svc ticketsvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "Inspect ticket responsibility groups (tenant-wide)",
	}
	cmd.AddCommand(newGroupsListCmd(svc))
	return cmd
}

func newGroupsListCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ticket responsibility groups",
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
			return runGroupsList(cmd.Context(), cmd.OutOrStdout(), s, profile, jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runGroupsList(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, jsonOut bool) error {
	groups, err := svc.ListGroups(ctx, profile)
	if err != nil {
		return err
	}
	if jsonOut {
		type groupJSON struct {
			ID     int    `json:"id"`
			Name   string `json:"name"`
			Active bool   `json:"active"`
		}
		out := make([]groupJSON, 0, len(groups))
		for _, g := range groups {
			out = append(out, groupJSON{ID: g.ID, Name: g.Name, Active: g.Active})
		}
		return render.JSON(w, struct {
			Schema string      `json:"schema"`
			Groups []groupJSON `json:"groups"`
		}{Schema: "tdx.v1.ticketGroupList", Groups: out})
	}
	if len(groups) == 0 {
		_, _ = fmt.Fprintln(w, "no ticket groups found")
		return nil
	}
	headers := []string{"ID", "NAME", "ACTIVE"}
	rows := make([][]string, 0, len(groups))
	for _, g := range groups {
		active := "yes"
		if !g.Active {
			active = "no"
		}
		rows = append(rows, []string{strconv.Itoa(g.ID), g.Name, active})
	}
	render.Table(w, headers, rows, nil)
	return nil
}
