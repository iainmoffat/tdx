package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathCmd_PrintsResolvedPaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"path"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), dir)
	require.Contains(t, out.String(), "config.yaml")
	require.Contains(t, out.String(), "credentials.yaml")
	require.Contains(t, out.String(), "templates")
}

func TestInitCmd_CreatesConfigDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "tdx")
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"init"})
	require.NoError(t, cmd.Execute())

	info, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestShowCmd_ReportsEmptyWhenNoConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), "no profiles configured")
}
