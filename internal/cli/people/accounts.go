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

func newAccountsCmd(svc peoplesvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "TD accounts/departments",
	}
	cmd.AddCommand(newAccountsListCmd(svc))
	return cmd
}

func newAccountsListCmd(svc peoplesvcAPI) *cobra.Command {
	var (
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List TD accounts/departments",
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
			return runAccountsList(cmd.Context(), cmd.OutOrStdout(), people, profile, jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

// runAccountsList fetches and prints the account catalog. Pure function.
func runAccountsList(ctx context.Context, w io.Writer, svc peoplesvcAPI, profile string, jsonFlag bool) error {
	accounts, err := svc.SearchAccounts(ctx, profile)
	if err != nil {
		return err
	}
	sort.SliceStable(accounts, func(i, j int) bool {
		return accounts[i].Name < accounts[j].Name
	})

	if jsonFlag {
		type accountJSON struct {
			ID              int    `json:"id"`
			Name            string `json:"name"`
			IsActive        bool   `json:"isActive"`
			Code            string `json:"code,omitempty"`
			ManagerUID      string `json:"managerUID,omitempty"`
			ManagerFullName string `json:"managerFullName,omitempty"`
		}
		out := make([]accountJSON, 0, len(accounts))
		for _, a := range accounts {
			out = append(out, accountJSON{
				ID:              a.ID,
				Name:            a.Name,
				IsActive:        a.IsActive,
				Code:            a.Code,
				ManagerUID:      a.ManagerUID,
				ManagerFullName: a.ManagerFullName,
			})
		}
		return render.JSON(w, struct {
			Schema   string        `json:"schema"`
			Accounts []accountJSON `json:"accounts"`
		}{
			Schema:   "tdx.v1.accountList",
			Accounts: out,
		})
	}

	if len(accounts) == 0 {
		_, _ = fmt.Fprintln(w, "no accounts")
		return nil
	}

	headers := []string{"ID", "NAME", "MANAGER", "ACTIVE"}
	rows := make([][]string, 0, len(accounts))
	for _, a := range accounts {
		rows = append(rows, []string{
			fmt.Sprintf("%d", a.ID),
			a.Name,
			a.ManagerFullName,
			boolStr(a.IsActive),
		})
	}
	render.Table(w, headers, rows, nil)
	return nil
}
