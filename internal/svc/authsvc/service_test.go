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

type harness struct {
	paths  config.Paths
	srv    *httptest.Server
	tenant string
	svc    *Service
}

func newHarness(t *testing.T, handler http.HandlerFunc) *harness {
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

	svc := New(paths)
	return &harness{paths: paths, srv: srv, tenant: srv.URL, svc: svc}
}

func TestService_LoginWritesTokenAndProfile(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	sess, err := h.svc.Login(context.Background(), LoginInput{
		ProfileName:   "ufl-test",
		TenantBaseURL: h.tenant,
		Token:         "good-token",
	})
	require.NoError(t, err)
	require.Equal(t, "ufl-test", sess.Profile.Name)
	require.Equal(t, "good-token", sess.Token)

	// Verify the token was actually persisted.
	creds := config.NewCredentialsStore(h.paths)
	stored, err := creds.GetToken("ufl-test")
	require.NoError(t, err)
	require.Equal(t, "good-token", stored)
}

func TestService_LoginRejectsBadToken(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := h.svc.Login(context.Background(), LoginInput{
		ProfileName:   "ufl-test",
		TenantBaseURL: h.tenant,
		Token:         "nope",
	})
	require.ErrorIs(t, err, domain.ErrInvalidToken)

	// Nothing should have been written.
	creds := config.NewCredentialsStore(h.paths)
	_, err = creds.GetToken("ufl-test")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}

func TestService_StatusReportsNotAuthenticatedWhenNoToken(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {})

	profiles := config.NewProfileStore(h.paths)
	require.NoError(t, profiles.AddProfile(domain.Profile{
		Name:          "ufl-test",
		TenantBaseURL: h.tenant,
	}))

	status, err := h.svc.Status(context.Background(), "ufl-test")
	require.NoError(t, err)
	require.False(t, status.Authenticated)
	require.Equal(t, "ufl-test", status.Profile.Name)
}

func TestService_StatusVerifiesValidToken(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		case "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ReferenceID":42,"UID":"abcd-1234","FullName":"Iain Moffat","PrimaryEmail":"ipm@ufl.edu"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})

	_, err := h.svc.Login(context.Background(), LoginInput{
		ProfileName:   "ufl-test",
		TenantBaseURL: h.tenant,
		Token:         "good",
	})
	require.NoError(t, err)

	status, err := h.svc.Status(context.Background(), "ufl-test")
	require.NoError(t, err)
	require.True(t, status.Authenticated)
	require.True(t, status.TokenValid)
	require.Equal(t, "Iain Moffat", status.User.FullName)
	require.Equal(t, "ipm@ufl.edu", status.User.Email)
	require.Empty(t, status.UserErr)
}

func TestService_StatusFlagsExpiredToken(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	profiles := config.NewProfileStore(h.paths)
	require.NoError(t, profiles.AddProfile(domain.Profile{
		Name:          "ufl-test",
		TenantBaseURL: h.tenant,
	}))
	creds := config.NewCredentialsStore(h.paths)
	require.NoError(t, creds.SetToken("ufl-test", "stale"))

	status, err := h.svc.Status(context.Background(), "ufl-test")
	require.NoError(t, err)
	require.True(t, status.Authenticated, "token file exists")
	require.False(t, status.TokenValid, "but server rejects it")
}

func TestService_StatusNonFatalWhoAmIFailure(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		case "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})

	_, err := h.svc.Login(context.Background(), LoginInput{
		ProfileName:   "ufl-test",
		TenantBaseURL: h.tenant,
		Token:         "good",
	})
	require.NoError(t, err)

	status, err := h.svc.Status(context.Background(), "ufl-test")
	require.NoError(t, err, "whoami failure must not fail Status")
	require.True(t, status.TokenValid)
	require.True(t, status.User.IsZero())
	require.NotEmpty(t, status.UserErr, "should carry a non-empty error string")
}

func TestService_LogoutClearsCredentialsOnly(t *testing.T) {
	h := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	})

	_, err := h.svc.Login(context.Background(), LoginInput{
		ProfileName:   "ufl-test",
		TenantBaseURL: h.tenant,
		Token:         "t",
	})
	require.NoError(t, err)

	require.NoError(t, h.svc.Logout("ufl-test"))

	// Profile still exists.
	profiles := config.NewProfileStore(h.paths)
	_, err = profiles.GetProfile("ufl-test")
	require.NoError(t, err)

	// Credentials are gone.
	creds := config.NewCredentialsStore(h.paths)
	_, err = creds.GetToken("ufl-test")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}
