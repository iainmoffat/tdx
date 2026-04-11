package config

import (
	"os"
	"runtime"
	"testing"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestCredentialsStore_MissingFileReturnsNoCredentials(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	_, err := s.GetToken("ufl")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}

func TestCredentialsStore_SetAndGetToken(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.SetToken("ufl", "abc123"))

	token, err := s.GetToken("ufl")
	require.NoError(t, err)
	require.Equal(t, "abc123", token)
}

func TestCredentialsStore_FileIsZeroSixHundred(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix perms not meaningful on Windows")
	}
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.SetToken("ufl", "abc123"))

	info, err := os.Stat(p.CredentialsFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestCredentialsStore_OverwriteRekeys(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.SetToken("ufl", "first"))
	require.NoError(t, s.SetToken("ufl", "second"))

	token, err := s.GetToken("ufl")
	require.NoError(t, err)
	require.Equal(t, "second", token)
}

func TestCredentialsStore_MultipleProfiles(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.SetToken("ufl", "token-ufl"))
	require.NoError(t, s.SetToken("sandbox", "token-sandbox"))

	tu, err := s.GetToken("ufl")
	require.NoError(t, err)
	require.Equal(t, "token-ufl", tu)

	ts, err := s.GetToken("sandbox")
	require.NoError(t, err)
	require.Equal(t, "token-sandbox", ts)
}

func TestCredentialsStore_ClearToken(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.SetToken("ufl", "abc"))
	require.NoError(t, s.ClearToken("ufl"))

	_, err := s.GetToken("ufl")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}

func TestCredentialsStore_ClearMissingIsNoop(t *testing.T) {
	p := writablePaths(t)
	s := NewCredentialsStore(p)

	require.NoError(t, s.ClearToken("never-existed"))
}
