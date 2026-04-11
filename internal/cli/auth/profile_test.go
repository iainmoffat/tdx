package auth

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProfileAdd_AddsAndPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"profile", "add", "default", "--url", "https://ufl.teamdynamix.com/"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "added profile \"default\"")
}

func TestProfileList_ShowsAddedProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", "https://ufl.teamdynamix.com/"})
	require.NoError(t, cmd.Execute())

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"profile", "list"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "default")
	require.Contains(t, out.String(), "ufl.teamdynamix.com")
	require.Contains(t, out.String(), "*") // default marker
}

func TestProfileRemove_RemovesNamedProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	for _, name := range []string{"a", "b"} {
		cmd := NewCmd()
		cmd.SetArgs([]string{"profile", "add", name, "--url", "https://" + name + ".teamdynamix.com/"})
		require.NoError(t, cmd.Execute())
	}

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "remove", "a"})
	require.NoError(t, cmd.Execute())

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"profile", "list"})
	require.NoError(t, cmd.Execute())
	require.NotContains(t, out.String(), "a.teamdynamix.com")
	require.Contains(t, out.String(), "b.teamdynamix.com")
}

func TestProfileUse_SwitchesDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	for _, name := range []string{"first", "second"} {
		cmd := NewCmd()
		cmd.SetArgs([]string{"profile", "add", name, "--url", "https://" + name + ".teamdynamix.com/"})
		require.NoError(t, cmd.Execute())
	}

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "use", "second"})
	require.NoError(t, cmd.Execute())

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"profile", "list"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "* second")
}
