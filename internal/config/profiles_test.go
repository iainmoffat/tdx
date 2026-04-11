package config

import (
	"path/filepath"
	"testing"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func writablePaths(t *testing.T) Paths {
	t.Helper()
	dir := t.TempDir()
	return Paths{
		Root:            dir,
		ConfigFile:      filepath.Join(dir, "config.yaml"),
		CredentialsFile: filepath.Join(dir, "credentials.yaml"),
		TemplatesDir:    filepath.Join(dir, "templates"),
	}
}

func TestProfileStore_LoadEmptyReturnsNothing(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	cfg, err := s.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.Profiles)
	require.Equal(t, "", cfg.DefaultProfile)
}

func TestProfileStore_RoundTrip(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	in := ProfileConfig{
		DefaultProfile: "ufl",
		Profiles: []domain.Profile{
			{Name: "ufl", TenantBaseURL: "https://ufl.teamdynamix.com/"},
			{Name: "sandbox", TenantBaseURL: "https://sandbox.teamdynamix.com/"},
		},
	}
	require.NoError(t, s.Save(in))

	out, err := s.Load()
	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestProfileStore_AddProfile(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	err := s.AddProfile(domain.Profile{Name: "ufl", TenantBaseURL: "https://ufl.teamdynamix.com/"})
	require.NoError(t, err)

	cfg, err := s.Load()
	require.NoError(t, err)
	require.Len(t, cfg.Profiles, 1)
	require.Equal(t, "ufl", cfg.DefaultProfile, "first profile becomes default")
}

func TestProfileStore_AddDuplicateRejected(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	prof := domain.Profile{Name: "ufl", TenantBaseURL: "https://ufl.teamdynamix.com/"}
	require.NoError(t, s.AddProfile(prof))
	err := s.AddProfile(prof)
	require.ErrorIs(t, err, domain.ErrProfileExists)
}

func TestProfileStore_RemoveProfile(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	require.NoError(t, s.AddProfile(domain.Profile{Name: "a", TenantBaseURL: "https://a.teamdynamix.com/"}))
	require.NoError(t, s.AddProfile(domain.Profile{Name: "b", TenantBaseURL: "https://b.teamdynamix.com/"}))
	require.NoError(t, s.SetDefault("a"))

	require.NoError(t, s.RemoveProfile("a"))

	cfg, err := s.Load()
	require.NoError(t, err)
	require.Len(t, cfg.Profiles, 1)
	require.Equal(t, "b", cfg.Profiles[0].Name)
	require.Equal(t, "b", cfg.DefaultProfile, "default rolls over to remaining profile")
}

func TestProfileStore_RemoveMissing(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	err := s.RemoveProfile("nope")
	require.ErrorIs(t, err, domain.ErrProfileNotFound)
}

func TestProfileStore_GetProfile(t *testing.T) {
	p := writablePaths(t)
	s := NewProfileStore(p)

	prof := domain.Profile{Name: "ufl", TenantBaseURL: "https://ufl.teamdynamix.com/"}
	require.NoError(t, s.AddProfile(prof))

	got, err := s.GetProfile("ufl")
	require.NoError(t, err)
	require.Equal(t, prof, got)
}
