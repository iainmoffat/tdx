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
