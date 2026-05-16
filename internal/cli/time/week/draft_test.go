package week

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestParseDraftRef(t *testing.T) {
	cases := []struct {
		in       string
		wantDate string
		wantName string
		wantErr  bool
	}{
		{"2026-05-04", "2026-05-03", "default", false}, // any date in week → returns Sunday weekStart
		{"2026-05-03", "2026-05-03", "default", false}, // Sunday itself
		{"2026-05-04/pristine", "2026-05-03", "pristine", false},
		{"bogus", "", "", true},
		{"2026-05-04/", "", "", true},
		{"", "", "", true},
	}
	for _, c := range cases {
		date, name, err := ParseDraftRef(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v, wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if c.wantErr {
			continue
		}
		if got := date.Format("2006-01-02"); got != c.wantDate {
			t.Errorf("%s: date=%s, want %s", c.in, got, c.wantDate)
		}
		if name != c.wantName {
			t.Errorf("%s: name=%q, want %q", c.in, name, c.wantName)
		}
	}
}

func TestResolveWeekRef_EmptyDefaultsToCurrentWeek(t *testing.T) {
	weekStart, name, err := ResolveWeekRef("")
	require.NoError(t, err)
	require.Equal(t, "default", name)
	expected := domain.WeekRefContaining(time.Now()).StartDate
	require.Equal(t, expected, weekStart)
}

func TestResolveWeekRef_BareDate(t *testing.T) {
	weekStart, name, err := ResolveWeekRef("2026-05-04")
	require.NoError(t, err)
	require.Equal(t, "default", name)
	expected, _ := time.ParseInLocation("2006-01-02", "2026-05-03", domain.EasternTZ)
	require.Equal(t, expected, weekStart)
}

func TestResolveWeekRef_DateWithName(t *testing.T) {
	weekStart, name, err := ResolveWeekRef("2026-05-04/pristine")
	require.NoError(t, err)
	require.Equal(t, "pristine", name)
	expected, _ := time.ParseInLocation("2006-01-02", "2026-05-03", domain.EasternTZ)
	require.Equal(t, expected, weekStart)
}

func TestResolveWeekRef_InvalidDate(t *testing.T) {
	_, _, err := ResolveWeekRef("not-a-date")
	require.Error(t, err)
}

func TestResolveWeekRef_EmptyNameAfterSlash(t *testing.T) {
	_, _, err := ResolveWeekRef("2026-05-04/")
	require.Error(t, err, "empty name after slash should fail (delegates to ParseDraftRef)")
}

func TestSingleArgWeekCommands_AcceptZeroArgs(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"pull", newPullCmd()},
		{"status", newStatusCmd()},
		{"refresh", newRefreshCmd()},
		{"new", newNewCmd()},
		{"history", newHistoryCmd()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.cmd)
			require.NoError(t, tc.cmd.Args(tc.cmd, []string{}),
				"%s should accept zero args after migration", tc.name)
			require.NoError(t, tc.cmd.Args(tc.cmd, []string{"2026-05-04"}),
				"%s should still accept one arg", tc.name)
		})
	}
}

func TestParseDraftRef_RejectsInvalidName(t *testing.T) {
	_, _, err := ParseDraftRef("2026-04-12/../../foo")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}
