package auth

import (
	"bytes"
	"io"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestProfileAdd_AddsAndPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"profile", "add", "default", "--url", "https://demotemplate.teamdynamix.com/"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "added profile \"default\"")
}

func TestProfileList_ShowsAddedProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TDX_CONFIG_HOME", dir)

	cmd := NewCmd()
	cmd.SetArgs([]string{"profile", "add", "default", "--url", "https://demotemplate.teamdynamix.com/"})
	require.NoError(t, cmd.Execute())

	var out bytes.Buffer
	cmd = NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"profile", "list"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "default")
	require.Contains(t, out.String(), "demotemplate.teamdynamix.com")
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

func TestProfileUse_RejectsInvalidName(t *testing.T) {
	cmd := newProfileCmd()
	cmd.SetArgs([]string{"use", ".."})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}
