package ticket

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/ticketsvc"
)

func newAppCmd(svc ticketsvcAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Discover and select the default ticket app for this profile",
	}
	cmd.AddCommand(newAppListCmd(svc))
	cmd.AddCommand(newAppUseCmd())
	cmd.AddCommand(newAppShowCmd())
	return cmd
}

func newAppListCmd(svc ticketsvcAPI) *cobra.Command {
	var (
		jsonFlag    bool
		profileFlag string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List ticket apps in the tenant",
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
			return runAppList(cmd.Context(), cmd.OutOrStdout(), s, profile, jsonFlag)
		},
	}
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runAppList(ctx context.Context, w io.Writer, svc ticketsvcAPI, profile string, jsonOut bool) error {
	apps, err := svc.ListApps(ctx, profile)
	if err != nil {
		return err
	}
	if jsonOut {
		type appJSON struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
			Active      bool   `json:"active"`
			AppType     string `json:"appType,omitempty"`
		}
		out := make([]appJSON, 0, len(apps))
		for _, a := range apps {
			out = append(out, appJSON{ID: a.ID, Name: a.Name, Description: a.Description, Active: a.Active, AppType: a.AppType})
		}
		return render.JSON(w, struct {
			Schema string    `json:"schema"`
			Apps   []appJSON `json:"apps"`
		}{Schema: "tdx.v1.ticketAppList", Apps: out})
	}
	if len(apps) == 0 {
		_, _ = fmt.Fprintln(w, "no ticket apps found (try a tenant admin if you expected one)")
		return nil
	}
	headers := []string{"ID", "NAME", "DESCRIPTION", "ACTIVE"}
	rows := make([][]string, 0, len(apps))
	for _, a := range apps {
		active := "yes"
		if !a.Active {
			active = "no"
		}
		rows = append(rows, []string{strconv.Itoa(a.ID), a.Name, a.Description, active})
	}
	render.Table(w, headers, rows, nil)
	return nil
}

func newAppUseCmd() *cobra.Command {
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "use <id>",
		Short: "Set the default ticket app for the current profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("app id must be a positive integer, got %q", args[0])
			}
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}
			store := config.NewProfileStore(paths)
			return runAppUse(cmd.OutOrStdout(), store, profile, id)
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

// profileStoreAPI is the subset of *config.ProfileStore used by runAppUse/runAppShow.
type profileStoreAPI interface {
	GetProfile(name string) (domain.Profile, error)
	UpdateProfile(p domain.Profile) error
}

func runAppUse(w io.Writer, store profileStoreAPI, profile string, appID int) error {
	prof, err := store.GetProfile(profile)
	if err != nil {
		return fmt.Errorf("read profile %q: %w", profile, err)
	}
	prof.TicketAppID = appID
	if err := store.UpdateProfile(prof); err != nil {
		return fmt.Errorf("save profile %q: %w", profile, err)
	}
	_, _ = fmt.Fprintf(w, "ticket app set: profile %q → app %d\n", profile, appID)
	return nil
}

func newAppShowCmd() *cobra.Command {
	var profileFlag string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the current default ticket app for this profile",
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
			store := config.NewProfileStore(paths)
			return runAppShow(cmd.OutOrStdout(), store, profile)
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name")
	return cmd
}

func runAppShow(w io.Writer, store profileStoreAPI, profile string) error {
	prof, err := store.GetProfile(profile)
	if err != nil {
		return err
	}
	if prof.TicketAppID == 0 {
		_, _ = fmt.Fprintf(w, "profile %q has no ticket app set (run `tdx ticket app list` then `tdx ticket app use <id>`)\n", profile)
		return nil
	}
	_, _ = fmt.Fprintf(w, "profile %q ticket app: %d\n", profile, prof.TicketAppID)
	return nil
}
