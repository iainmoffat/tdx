package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ipm/tdx/internal/config"
	"github.com/stretchr/testify/require"
)

// loginRunner lets tests inject a fake token reader instead of prompting on tty.
type loginRunner struct {
	input string
}

func (l loginRunner) ReadToken(prompt string) (string, error) {
	return l.input, nil
}

func TestLogin_PasteTokenSuccess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer good", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	cmd := NewCmdWithTokenReader(loginRunner{input: "good"})
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"login", "--profile", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "signed in as profile \"default\"")

	p, err := config.ResolvePaths()
	require.NoError(t, err)
	creds := config.NewCredentialsStore(p)
	token, err := creds.GetToken("default")
	require.NoError(t, err)
	require.Equal(t, "good", token)
}

func TestLogin_EmptyTokenRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmdWithTokenReader(loginRunner{input: "   "})
	cmd.SetArgs([]string{"login", "--profile", "default", "--url", "https://ufl.teamdynamix.com/"})
	err := cmd.Execute()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "empty token") || strings.Contains(err.Error(), "invalid token"))
}

func TestLogin_ServerRejectsToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cmd := NewCmdWithTokenReader(loginRunner{input: "bad"})
	cmd.SetArgs([]string{"login", "--profile", "default", "--url", srv.URL})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid token")

	p, err := config.ResolvePaths()
	require.NoError(t, err)
	creds := config.NewCredentialsStore(p)
	_, err = creds.GetToken("default")
	require.Error(t, err, "no token should be written on failure")
}

func TestStdinReader_ReadToken_TrimsWhitespace(t *testing.T) {
	r := stdinReader{in: strings.NewReader("  abc-123-token  \n")}
	got, err := r.ReadToken("ignored prompt")
	require.NoError(t, err)
	require.Equal(t, "abc-123-token", got)
}

func TestStdinReader_ReadToken_EmptyInputErrors(t *testing.T) {
	r := stdinReader{in: strings.NewReader("")}
	_, err := r.ReadToken("ignored")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty token")
}

func TestStdinReader_ReadToken_HandlesMultilineByTakingFirstLine(t *testing.T) {
	r := stdinReader{in: strings.NewReader("line-one-token\nline-two-noise\n")}
	got, err := r.ReadToken("ignored")
	require.NoError(t, err)
	require.Equal(t, "line-one-token", got)
}

// fakeReader is a no-op TokenReader that returns a canned token. Used when
// the test cares about flag parsing or browser launch but not the TTY flow.
type fakeReader struct {
	token string
}

func (f fakeReader) ReadToken(prompt string) (string, error) {
	return f.token, nil
}

// panicReader is a TokenReader that panics if ReadToken is called. Used in
// --token-stdin tests to assert the TTY path is NOT taken — if the flow
// accidentally falls through to the TTY reader, the test fails loudly
// instead of silently passing with the wrong token source.
type panicReader struct{}

func (panicReader) ReadToken(prompt string) (string, error) {
	panic("panicReader.ReadToken called — --token-stdin flow incorrectly fell back to TTY")
}

// TestLogin_SSOFlag verifies that --sso opens the browser to the loginsso URL
// and that the existing token validation flow still runs after the browser launch.
func TestLogin_SSOFlag(t *testing.T) {
	// Override openBrowser so the test doesn't launch a real browser.
	original := openBrowser
	defer func() { openBrowser = original }()
	var openedURL string
	openBrowser = func(url string) error {
		openedURL = url
		return nil
	}

	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	// Mock TD server that accepts any token and returns 200 on /api/time/types.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	// Use the auth-tree entry point with a fake TTY reader.
	cmd := NewCmdWithTokenReader(fakeReader{token: "valid-token"})
	cmd.SetArgs([]string{"login", "--sso", "--profile", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	require.Equal(t, srv.URL+"/TDWebApi/api/auth/loginsso", openedURL,
		"--sso must open the loginsso URL in the browser")
}

// TestLogin_TokenStdinFlag verifies that --token-stdin reads the token from
// the package-level stdinSource var instead of the TTY reader.
func TestLogin_TokenStdinFlag(t *testing.T) {
	// Override stdinSource so the test injects a fake stdin without touching os.Stdin.
	originalStdin := stdinSource
	defer func() { stdinSource = originalStdin }()
	stdinSource = strings.NewReader("stdin-token\n")

	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer stdin-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	// Use the existing NewCmdWithTokenReader entry point with a panicking
	// TTY reader — if the flow accidentally falls back to TTY, we'll see it.
	cmd := NewCmdWithTokenReader(panicReader{})
	cmd.SetArgs([]string{"login", "--token-stdin", "--profile", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())
}

// TestLogin_TokenStdinAndSSOCombined verifies that both flags can be used
// together: open the browser AND read from stdin (the scripted SSO flow).
func TestLogin_TokenStdinAndSSOCombined(t *testing.T) {
	originalBrowser := openBrowser
	defer func() { openBrowser = originalBrowser }()
	var openedURL string
	openBrowser = func(url string) error {
		openedURL = url
		return nil
	}

	originalStdin := stdinSource
	defer func() { stdinSource = originalStdin }()
	stdinSource = strings.NewReader("combined-token\n")

	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	cmd := NewCmdWithTokenReader(panicReader{})
	cmd.SetArgs([]string{"login", "--sso", "--token-stdin", "--profile", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	require.Contains(t, openedURL, "loginsso", "browser should still open with --sso")
}
