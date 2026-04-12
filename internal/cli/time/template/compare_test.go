package template

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCompareCmd_Basic verifies that compare runs the reconciliation engine
// and prints the annotated grid preview without writing anything.
func TestCompareCmd_Basic(t *testing.T) {
	srv, postCount := setupApplyServer(t, false)
	defer srv.Close()

	setupApplyEnv(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"compare", "test-tmpl", "--week", "2026-04-05"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	// Must show the preview grid.
	require.Contains(t, got, "Apply preview:", "output should show grid preview title")
	// Must show the summary line.
	require.Contains(t, got, "to create", "output should include summary with creates")
	// Must never POST — compare is always read-only.
	require.Equal(t, int32(0), postCount.Load(), "compare must not POST time entries")
}

// TestCompareCmd_MissingWeek ensures --week is required.
func TestCompareCmd_MissingWeek(t *testing.T) {
	_ = seedTemplateDir(t)

	cmd := NewCmd()
	cmd.SetArgs([]string{"compare", "test-tmpl"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--week is required")
}

// TestCompareCmd_JSONFlag verifies JSON output includes diffHash and the
// compare-specific schema string.
func TestCompareCmd_JSONFlag(t *testing.T) {
	srv, postCount := setupApplyServer(t, false)
	defer srv.Close()

	setupApplyEnv(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"compare", "test-tmpl", "--week", "2026-04-05", "--json"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "diffHash", "JSON output should include diffHash")
	require.Contains(t, got, "tdx.v1.templateComparePreview", "JSON schema should identify compare")

	// Validate it is parseable JSON.
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &payload))
	require.Equal(t, int32(0), postCount.Load(), "compare --json must not POST")
}

// TestCompareCmd_DaysFilter confirms that --days restricts which entries show
// up in the planned creates.
func TestCompareCmd_DaysFilter(t *testing.T) {
	srv, _ := setupApplyServer(t, false)
	defer srv.Close()

	setupApplyEnv(t, srv.URL)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"compare", "test-tmpl", "--week", "2026-04-05", "--days", "mon,tue"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "2 to create", "should show 2 creates for mon,tue")
}

// TestCompareCmd_NoYesFlag verifies that compare does not expose a --yes flag.
func TestCompareCmd_NoYesFlag(t *testing.T) {
	cmd := NewCmd()
	compareCmd, _, err := cmd.Find([]string{"compare"})
	require.NoError(t, err)
	require.NotNil(t, compareCmd)

	require.Nil(t, compareCmd.Flags().Lookup("yes"), "compare must not have a --yes flag")
	require.Nil(t, compareCmd.Flags().Lookup("dry-run"), "compare must not have a --dry-run flag")
}

// TestCompareCmd_InvalidWeek ensures a bad date is rejected with a clear error.
func TestCompareCmd_InvalidWeek(t *testing.T) {
	_ = seedTemplateDir(t)

	cmd := NewCmd()
	cmd.SetArgs([]string{"compare", "test-tmpl", "--week", "not-a-date"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid --week")
}

// TestCompareCmd_InvalidDays ensures an unknown day abbreviation is rejected.
func TestCompareCmd_InvalidDays(t *testing.T) {
	srv, _ := setupApplyServer(t, false)
	defer srv.Close()

	setupApplyEnv(t, srv.URL)

	cmd := NewCmd()
	cmd.SetArgs([]string{"compare", "test-tmpl", "--week", "2026-04-05", "--days", "badday"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid --days")
}

// TestCompareCmd_InvalidOverride ensures a malformed --override is rejected.
func TestCompareCmd_InvalidOverride(t *testing.T) {
	srv, _ := setupApplyServer(t, false)
	defer srv.Close()

	setupApplyEnv(t, srv.URL)

	cmd := NewCmd()
	cmd.SetArgs([]string{"compare", "test-tmpl", "--week", "2026-04-05", "--override", "bad-format"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid --override")
}
