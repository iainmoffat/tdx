package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatus_NoProfileConfigured(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"status"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, out.String()+err.Error(), "no profile")
}

func TestStatus_ProfileWithoutToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "not authenticated")
}

func TestStatus_ProfileWithValidToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer srv.Close()

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	setTokenForTest(t, "default", "good-token")

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "authenticated")
	require.Contains(t, out.String(), "token:    valid")
	require.Contains(t, out.String(), "user:     Iain Moffat")
	require.Contains(t, out.String(), "email:    ipm@ufl.edu")
}

func TestStatus_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	defer srv.Close()

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", srv.URL})
	require.NoError(t, cmd.Execute())

	setTokenForTest(t, "default", "good-token")

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--json"})
	require.NoError(t, cmd.Execute())

	s := out.String()
	require.Contains(t, s, `"profile": "default"`)
	require.Contains(t, s, `"authenticated": true`)
	require.Contains(t, s, `"tokenValid": true`)
	require.Contains(t, s, `"fullName": "Iain Moffat"`)
	require.Contains(t, s, `"email": "ipm@ufl.edu"`)
}
