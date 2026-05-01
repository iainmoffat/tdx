package people

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

func newPoolsCmd(svc peoplesvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pools",
		Short: "TD resource pools",
	}
	cmd.AddCommand(newPoolsListCmd(svc))
	return cmd
}

func newPoolsListCmd(svc peoplesvcAPI) *cobra.Command {
	var (
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List TD resource pools",
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
			people := svc
			if people == nil {
				people = peoplesvc.New(paths)
			}
			return runPoolsList(cmd.Context(), cmd.OutOrStdout(), people, profile, jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

// runPoolsList fetches and prints the resource pool list. Pure function:
// no config resolution, no global state. Tests inject a stub svc.
func runPoolsList(ctx context.Context, w io.Writer, svc peoplesvcAPI, profile string, jsonFlag bool) error {
	pools, err := svc.SearchPools(ctx, profile)
	if err != nil {
		return err
	}
	sort.SliceStable(pools, func(i, j int) bool {
		return pools[i].Name < pools[j].Name
	})

	if jsonFlag {
		type poolJSON struct {
			ID               int    `json:"id"`
			Name             string `json:"name"`
			IsActive         bool   `json:"isActive"`
			RequiresApproval bool   `json:"requiresApproval"`
			ManagerUID       string `json:"managerUID,omitempty"`
			ManagerFullName  string `json:"managerFullName,omitempty"`
		}
		out := make([]poolJSON, 0, len(pools))
		for _, p := range pools {
			out = append(out, poolJSON{
				ID:               p.ID,
				Name:             p.Name,
				IsActive:         p.IsActive,
				RequiresApproval: p.RequiresApproval,
				ManagerUID:       p.ManagerUID,
				ManagerFullName:  p.ManagerFullName,
			})
		}
		return render.JSON(w, struct {
			Schema string     `json:"schema"`
			Pools  []poolJSON `json:"pools"`
		}{
			Schema: "tdx.v1.resourcePoolList",
			Pools:  out,
		})
	}

	if len(pools) == 0 {
		_, _ = fmt.Fprintln(w, "no resource pools")
		return nil
	}

	headers := []string{"ID", "NAME", "MANAGER", "REQ-APPROVAL", "ACTIVE"}
	rows := make([][]string, 0, len(pools))
	for _, p := range pools {
		rows = append(rows, []string{
			fmt.Sprintf("%d", p.ID),
			p.Name,
			p.ManagerFullName,
			boolStr(p.RequiresApproval),
			boolStr(p.IsActive),
		})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
