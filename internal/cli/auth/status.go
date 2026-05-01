package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/spf13/cobra"
)

// statusJSON is the stable JSON shape emitted by `tdx auth status --json`.
// Part of the tdx.v1 schema per spec §9.
type statusJSON struct {
	Profile        string `json:"profile"`
	Tenant         string `json:"tenant"`
	Authenticated  bool   `json:"authenticated"`
	TokenValid     bool   `json:"tokenValid"`
	TokenExpiresAt string `json:"tokenExpiresAt,omitempty"`
	Error          string `json:"error,omitempty"`
	FullName       string `json:"fullName,omitempty"`
	Email          string `json:"email,omitempty"`
	UserError      string `json:"userError,omitempty"`
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
				expiresAt := ""
				if !status.TokenExpiresAt.IsZero() {
					expiresAt = status.TokenExpiresAt.Format(time.RFC3339)
				}
				return render.JSON(cmd.OutOrStdout(), statusJSON{
					Profile:        status.Profile.Name,
					Tenant:         status.Profile.TenantBaseURL,
					Authenticated:  status.Authenticated,
					TokenValid:     status.TokenValid,
					TokenExpiresAt: expiresAt,
					Error:          status.ValidationErr,
					FullName:       status.User.FullName,
					Email:          status.User.Email,
					UserError:      status.UserErr,
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
			if !status.TokenExpiresAt.IsZero() {
				_, _ = fmt.Fprintf(w, "expires:  %s (%s)\n",
					status.TokenExpiresAt.Local().Format("2006-01-02 15:04 MST"),
					formatTimeUntil(time.Until(status.TokenExpiresAt)))
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

// formatTimeUntil renders a duration into a friendly "in 5h", "expired 1h ago"
// form. Server-side ping already covers the actual valid/invalid verdict;
// this is just informational alongside the absolute expiry timestamp.
func formatTimeUntil(d time.Duration) string {
	if d < 0 {
		return "expired " + roundDuration(-d) + " ago"
	}
	return "in " + roundDuration(d)
}

func roundDuration(d time.Duration) string {
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%.0fh", d.Hours())
	case d >= time.Minute:
		return fmt.Sprintf("%.0fm", d.Minutes())
	default:
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
}
