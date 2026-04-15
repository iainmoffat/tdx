package auth

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
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

// stdinSource is the io.Reader that stdinReader reads from when --token-stdin
// is set. Production points it at os.Stdin; tests override it via save/restore.
// Mirrors the openBrowser package var pattern from browser.go.
var stdinSource io.Reader = os.Stdin

// stdinReader reads a token from an io.Reader (typically os.Stdin) without
// any TTY check. Used by --token-stdin for scripted/CI use where there is
// no terminal. Reads the FIRST line only and trims surrounding whitespace,
// so a trailing newline from `echo "$TOKEN"` is handled cleanly.
type stdinReader struct {
	in io.Reader
}

func (r stdinReader) ReadToken(prompt string) (string, error) {
	scanner := bufio.NewScanner(r.in)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read token from stdin: %w", err)
		}
		return "", fmt.Errorf("empty token on stdin")
	}
	tok := strings.TrimSpace(scanner.Text())
	if tok == "" {
		return "", fmt.Errorf("empty token on stdin")
	}
	return tok, nil
}

func newLoginCmd(reader TokenReader) *cobra.Command {
	var profileFlag, urlFlag string
	var ssoFlag, tokenStdinFlag bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in to TeamDynamix via paste-token, --sso, or --token-stdin",
		Long: `Paste-token login (default), or use --sso to open the TD SSO URL in
your browser, or --token-stdin to read the token from stdin (for scripted use).

Default flow: log in to TeamDynamix in your browser, navigate to your user
profile's API token view, copy the token, then run this command and paste the
token when prompted.

--sso flow: opens https://<tenant>/TDWebApi/api/auth/loginsso in your browser.
After SSO completes, copy the token shown and paste it here. The loginsso
endpoint issues a fresh 24-hour JWT each time it is called with a valid TD
session cookie.

--token-stdin flow: reads the token from stdin instead of prompting on the TTY.
Useful for CI/scripts: ` + "`" + `echo "$TOKEN" | tdx auth login --token-stdin --profile default --url https://...` + "`" + `

--sso and --token-stdin can be combined: open the browser, then read the token
from stdin.

The token is validated against the tenant's /TDWebApi/api/time/types endpoint
before being saved to ~/.config/tdx/credentials.yaml.`,
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
				return fmt.Errorf("--url is required (no existing profile to inherit from)\n  Example: tdx auth login --url https://yourorg.teamdynamix.com/")
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Signing in to %s as profile %q.\n", tenantURL, profileName)

			// --sso: open the browser to the loginsso URL (best effort).
			if ssoFlag {
				ssoURL := strings.TrimRight(tenantURL, "/") + "/TDWebApi/api/auth/loginsso"
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Opening %s in your browser.\n", ssoURL)
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Complete the SSO flow if prompted, then copy the token shown.")
				if err := openBrowser(ssoURL); err != nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "(could not open browser automatically: %s — please open the URL manually)\n", err)
				}
			}

			// Pick TokenReader: --token-stdin uses stdinReader, otherwise the
			// TTY reader passed in via newLoginCmd's parameter (which is what
			// the existing Phase 1 NewCmdWithTokenReader entry point already
			// does for fake-reader tests).
			activeReader := reader
			if tokenStdinFlag {
				activeReader = stdinReader{in: stdinSource}
			} else {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Paste your TD API token and press Enter:")
			}

			raw, err := activeReader.ReadToken("Token: ")
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
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "signed in as profile %q (%s)\n", sess.Profile.Name, sess.Profile.TenantBaseURL)
			return nil
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name to sign in as (default: existing default or 'default')")
	cmd.Flags().StringVar(&urlFlag, "url", "", "tenant base URL (required for first login; inherited from existing profile afterward)")
	cmd.Flags().BoolVar(&ssoFlag, "sso", false, "open the TD SSO URL in your browser before prompting for the token")
	cmd.Flags().BoolVar(&tokenStdinFlag, "token-stdin", false, "read the token from stdin instead of prompting on the TTY (for scripts/CI)")
	return cmd
}
