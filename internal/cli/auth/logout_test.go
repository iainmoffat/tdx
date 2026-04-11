package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestLogout_ClearsTokenButKeepsProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	setTokenForTest(t, "default", "abc")

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"logout"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "logged out")

	p, err := config.ResolvePaths()
	require.NoError(t, err)
	creds := config.NewCredentialsStore(p)
	_, err = creds.GetToken("default")
	require.ErrorIs(t, err, domain.ErrNoCredentials)

	store := config.NewProfileStore(p)
	_, err = store.GetProfile("default")
	require.NoError(t, err, "profile should remain")
}

func TestLogout_NoCredentialsIsStillSuccess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	cmd = NewCmd()
	cmd.SetArgs([]string{"logout"})
	require.NoError(t, cmd.Execute())
}
