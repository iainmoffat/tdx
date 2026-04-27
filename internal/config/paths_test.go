package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaths_RespectsTdxConfigHomeEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	p, err := ResolvePaths()
	require.NoError(t, err)
	require.Equal(t, dir, p.Root)
	require.Equal(t, filepath.Join(dir, "config.yaml"), p.ConfigFile)
	require.Equal(t, filepath.Join(dir, "credentials.yaml"), p.CredentialsFile)
	require.Equal(t, filepath.Join(dir, "templates"), p.TemplatesDir)
}

func TestPaths_FallsBackToXdgConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", dir)

	p, err := ResolvePaths()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "tdx"), p.Root)
}

func TestPaths_FallsBackToHomeDotConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)

	p, err := ResolvePaths()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".config", "tdx"), p.Root)
}

func TestProfilePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", home)
	p := MustPaths()

	if got, want := p.ProfileTemplatesDir("work"), filepath.Join(home, "profiles", "work", "templates"); got != want {
		t.Errorf("ProfileTemplatesDir(work) = %q, want %q", got, want)
	}
	if got, want := p.ProfileWeeksDir("work"), filepath.Join(home, "profiles", "work", "weeks"); got != want {
		t.Errorf("ProfileWeeksDir(work) = %q, want %q", got, want)
	}
	if got, want := p.LegacyTemplatesDir, filepath.Join(home, "templates"); got != want {
		t.Errorf("LegacyTemplatesDir = %q, want %q", got, want)
	}
}
