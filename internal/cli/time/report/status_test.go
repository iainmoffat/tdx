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
	for _, f := range []string{"week", "from", "to", "user", "manager", "account", "resource-pool", "all", "yes", "include-zero", "incomplete", "threshold", "limit", "json", "csv", "xlsx", "profile"} {
		require.NotNil(t, cmd.Flags().Lookup(f), "missing flag: %s", f)
	}
}

func TestStatus_ThresholdRequiresIncomplete(t *testing.T) {
	_, err := runStatusCmd(t, "--week", "2026-04-12", "--user", "u1", "--threshold", "32")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--threshold requires --incomplete")
}

func TestStatus_IncompleteWithDefaultThresholdAccepted(t *testing.T) {
	// Validation should accept --incomplete without --threshold. (Run path
	// errors later because no real config exists, but flag validation must
	// pass before that.)
	out, err := runStatusCmd(t, "--week", "2026-04-12", "--user", "u1", "--incomplete")
	require.Error(t, err, "expected error from execution (no profile), not validation")
	require.NotContains(t, err.Error(), "--threshold requires --incomplete")
	_ = out
}

func TestStatus_ResourcePoolIsValidSelector(t *testing.T) {
	f := statusFlags{
		resourcePools: []string{"ICT - DBP - Linux Platform Services LPS"},
		week:          "2026-04-12",
	}
	require.NoError(t, validateStatusFlags(f))
}

func TestStatus_ResourcePoolMutuallyExclusiveWithOthers(t *testing.T) {
	f := statusFlags{
		resourcePools: []string{"Some Pool"},
		managers:      []string{"me"},
		week:          "2026-04-12",
	}
	err := validateStatusFlags(f)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one")
}
