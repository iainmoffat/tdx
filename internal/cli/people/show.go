package people

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

func newShowCmd(svc peoplesvcAPI) *cobra.Command {
	var (
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "show <UID>",
		Short: "Show full details for a single user by UID",
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
			people := svc
			if people == nil {
				people = peoplesvc.New(paths)
			}
			return runPeopleShow(cmd.Context(), cmd.OutOrStdout(), people,
				profile, args[0], jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

// runPeopleShow is the pure implementation.
func runPeopleShow(ctx context.Context, w io.Writer, svc peoplesvcAPI,
	profile, uid string, jsonOut bool) error {
	user, err := svc.GetUser(ctx, profile, uid)
	if err != nil {
		return err
	}
	if jsonOut {
		return render.JSON(w, struct {
			Schema string      `json:"schema"`
			Person domain.User `json:"person"`
		}{
			Schema: "tdx.v1.person",
			Person: user,
		})
	}
	printPersonText(w, user)
	return nil
}

func printPersonText(w io.Writer, u domain.User) {
	_, _ = fmt.Fprintf(w, "UID:           %s\n", u.UID)
	_, _ = fmt.Fprintf(w, "Name:          %s\n", u.FullName)
	if u.Email != "" {
		_, _ = fmt.Fprintf(w, "Email:         %s\n", u.Email)
	}
	_, _ = fmt.Fprintf(w, "Active:        %s\n", boolStr(u.Active))
	_, _ = fmt.Fprintf(w, "Employee:      %s\n", boolStr(u.IsEmployee))
	if u.Title != "" {
		_, _ = fmt.Fprintf(w, "Title:         %s\n", u.Title)
	}
	if u.AccountName != "" {
		_, _ = fmt.Fprintf(w, "Account:       %s\n", u.AccountName)
	}
	if u.ResourcePoolName != "" {
		_, _ = fmt.Fprintf(w, "Resource pool: %s\n", u.ResourcePoolName)
	}
	if u.ReportsToName != "" {
		mgr := u.ReportsToName
		if u.ReportsToEmail != "" {
			mgr = fmt.Sprintf("%s <%s>", u.ReportsToName, u.ReportsToEmail)
		}
		_, _ = fmt.Fprintf(w, "Manager:       %s\n", mgr)
	}
}
