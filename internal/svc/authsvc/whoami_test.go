package authsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func whoamiHarness(t *testing.T, handler http.HandlerFunc) (*Service, string) {
	t.Helper()
	dir := t.TempDir()
	paths := config.Paths{
		Root:            dir,
		ConfigFile:      filepath.Join(dir, "config.yaml"),
		CredentialsFile: filepath.Join(dir, "credentials.yaml"),
		TemplatesDir:    filepath.Join(dir, "templates"),
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// Seed a profile + token for the service to use.
	ps := config.NewProfileStore(paths)
	require.NoError(t, ps.AddProfile(domain.Profile{
		Name:          "default",
		TenantBaseURL: srv.URL,
	}))
	cs := config.NewCredentialsStore(paths)
	require.NoError(t, cs.SetToken("default", "good-token"))

	return New(paths), "default"
}

func TestWhoAmI_DecodesUser(t *testing.T) {
	svc, profile := whoamiHarness(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/auth/getuser", r.URL.Path)
		require.Equal(t, "Bearer good-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ReferenceID": 42,
			"UID": "abcd-1234",
			"FullName": "Iain Moffat",
			"PrimaryEmail": "ipm@ufl.edu"
		}`))
	})

	user, err := svc.WhoAmI(context.Background(), profile)
	require.NoError(t, err)
	require.Equal(t, 42, user.ID)
	require.Equal(t, "abcd-1234", user.UID)
	require.Equal(t, "Iain Moffat", user.FullName)
	require.Equal(t, "ipm@ufl.edu", user.Email)
}

func TestWhoAmI_Unauthorized(t *testing.T) {
	svc, profile := whoamiHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := svc.WhoAmI(context.Background(), profile)
	require.Error(t, err)
}

func TestWhoAmI_UnknownProfile(t *testing.T) {
	svc, _ := whoamiHarness(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := svc.WhoAmI(context.Background(), "does-not-exist")
	require.ErrorIs(t, err, domain.ErrProfileNotFound)
}

func TestWhoAmI_FallsBackToAlternateEmail(t *testing.T) {
	svc, profile := whoamiHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ReferenceID": 7,
			"UID": "u-7",
			"FullName": "Alt User",
			"PrimaryEmail": "",
			"AlternateEmail": "alt@example.com"
		}`))
	})

	user, err := svc.WhoAmI(context.Background(), profile)
	require.NoError(t, err)
	require.Equal(t, "alt@example.com", user.Email)
}
