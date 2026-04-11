package auth

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/svc/authsvc"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// TokenReader abstracts how the CLI collects a token from the user.
// Production reads from a TTY without echo; tests inject a fake.
type TokenReader interface {
	ReadToken(prompt string) (string, error)
}

type ttyReader struct{}

func (ttyReader) ReadToken(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func newLoginCmd(reader TokenReader) *cobra.Command {
	var profileFlag, urlFlag string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in to TeamDynamix via paste-token flow",
		Long: `Paste-token login.

Log in to TeamDynamix in your browser, navigate to your user profile's
API token view, copy the token, then run this command and paste the
token when prompted. The token is validated against the tenant's
/api/time/types endpoint before being saved to ~/.config/tdx/credentials.yaml.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			svc := authsvc.New(paths)

			// Resolve profile name. If none given and none configured, require --profile.
			profileName := profileFlag
			if profileName == "" {
				profileName, _ = svc.ResolveProfile("")
			}
			if profileName == "" {
				profileName = "default"
			}

			// Resolve tenant URL. Priority: --url > existing profile > prompt.
			tenantURL := urlFlag
			if tenantURL == "" {
				if existing, err := svc.Profiles().GetProfile(profileName); err == nil {
					tenantURL = existing.TenantBaseURL
				}
			}
			if tenantURL == "" {
				tenantURL = "https://ufl.teamdynamix.com/"
				fmt.Fprintf(cmd.ErrOrStderr(), "no --url given; defaulting to %s\n", tenantURL)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Signing in to %s as profile %q.\n", tenantURL, profileName)
			fmt.Fprintln(cmd.ErrOrStderr(), "Open TeamDynamix in your browser, copy your API token, and paste it here.")

			raw, err := reader.ReadToken("Token: ")
			if err != nil {
				return err
			}
			token := strings.TrimSpace(raw)

			sess, err := svc.Login(context.Background(), authsvc.LoginInput{
				ProfileName:   profileName,
				TenantBaseURL: tenantURL,
				Token:         token,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "signed in as profile %q (%s)\n", sess.Profile.Name, sess.Profile.TenantBaseURL)
			return nil
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name to sign in as (default: existing default or 'default')")
	cmd.Flags().StringVar(&urlFlag, "url", "", "tenant base URL (default: existing profile or https://ufl.teamdynamix.com/)")
	return cmd
}
