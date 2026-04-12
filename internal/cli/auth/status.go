package auth

import (
	"context"
	"fmt"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/render"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/spf13/cobra"
)

// statusJSON is the stable JSON shape emitted by `tdx auth status --json`.
// Part of the tdx.v1 schema per spec §9.
type statusJSON struct {
	Profile       string `json:"profile"`
	Tenant        string `json:"tenant"`
	Authenticated bool   `json:"authenticated"`
	TokenValid    bool   `json:"tokenValid"`
	Error         string `json:"error,omitempty"`
	FullName      string `json:"fullName,omitempty"`
	Email         string `json:"email,omitempty"`
	UserError     string `json:"userError,omitempty"`
}

func newStatusCmd() *cobra.Command {
	var profileFlag string
	var jsonFlag bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the current auth state",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			svc := authsvc.New(paths)

			profileName, err := svc.ResolveProfile(profileFlag)
			if err != nil {
				return fmt.Errorf("no profile configured — run 'tdx auth login' or 'tdx auth profile add'")
			}

			status, err := svc.Status(context.Background(), profileName)
			if err != nil {
				return err
			}

			format := render.ResolveFormat(render.Flags{JSON: jsonFlag})
			if format == render.FormatJSON {
				return render.JSON(cmd.OutOrStdout(), statusJSON{
					Profile:       status.Profile.Name,
					Tenant:        status.Profile.TenantBaseURL,
					Authenticated: status.Authenticated,
					TokenValid:    status.TokenValid,
					Error:         status.ValidationErr,
					FullName:      status.User.FullName,
					Email:         status.User.Email,
					UserError:     status.UserErr,
				})
			}

			w := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(w, "profile:  %s\n", status.Profile.Name)
			_, _ = fmt.Fprintf(w, "tenant:   %s\n", status.Profile.TenantBaseURL)
			if !status.Authenticated {
				_, _ = fmt.Fprintln(w, "state:    not authenticated")
				_, _ = fmt.Fprintln(w, "          run 'tdx auth login' to sign in")
				return nil
			}
			_, _ = fmt.Fprintln(w, "state:    authenticated")
			if status.TokenValid {
				_, _ = fmt.Fprintln(w, "token:    valid")
			} else {
				_, _ = fmt.Fprintf(w, "token:    invalid (%s)\n", status.ValidationErr)
				_, _ = fmt.Fprintln(w, "          run 'tdx auth login' to refresh")
				return nil
			}

			// Identity lines — only when we have a valid token.
			if status.UserErr != "" {
				_, _ = fmt.Fprintf(w, "user:     (lookup failed: %s)\n", status.UserErr)
			} else if !status.User.IsZero() {
				_, _ = fmt.Fprintf(w, "user:     %s\n", status.User.DisplayName())
				if status.User.Email != "" {
					_, _ = fmt.Fprintf(w, "email:    %s\n", status.User.Email)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to the configured default)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit status as JSON")
	return cmd
}
