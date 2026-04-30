package report

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func runStatusCmd(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), err
}

func TestStatus_SelectorRequired(t *testing.T) {
	_, err := runStatusCmd(t, "--week", "2026-04-12")
	require.Error(t, err)
	require.Contains(t, err.Error(), "selector")
}

func TestStatus_SelectorMutuallyExclusive(t *testing.T) {
	_, err := runStatusCmd(t,
		"--week", "2026-04-12",
		"--user", "u1",
		"--manager", "me",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one")
}

func TestStatus_FormatMutuallyExclusive(t *testing.T) {
	_, err := runStatusCmd(t,
		"--week", "2026-04-12",
		"--user", "u1",
		"--json", "--csv",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "format")
}

func TestStatus_AllRequiresYes(t *testing.T) {
	_, err := runStatusCmd(t,
		"--week", "2026-04-12",
		"--all",
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--yes")
}

func TestStatus_FlagsRegistered(t *testing.T) {
	cmd := newStatusCmd()
	for _, f := range []string{"week", "from", "to", "user", "manager", "account", "all", "yes", "include-zero", "limit", "json", "csv", "xlsx", "profile"} {
		require.NotNil(t, cmd.Flags().Lookup(f), "missing flag: %s", f)
	}
}
