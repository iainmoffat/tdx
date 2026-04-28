package draftsvc

import (
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestStrategy_Validate(t *testing.T) {
	cases := []struct {
		in      Strategy
		wantErr bool
	}{
		{StrategyAbort, false},
		{StrategyOurs, false},
		{StrategyTheirs, false},
		{Strategy(""), true},
		{Strategy("merge"), true},
	}
	for _, tc := range cases {
		err := tc.in.Validate()
		if tc.wantErr {
			require.Error(t, err, "expected error for %q", tc.in)
		} else {
			require.NoError(t, err, "unexpected error for %q", tc.in)
		}
	}
}

func TestRefreshResult_ZeroValueIsAbortFalse(t *testing.T) {
	var r RefreshResult
	require.False(t, r.Aborted)
	require.Empty(t, r.Conflicts)
	require.Equal(t, 0, r.Adopted)
	require.Equal(t, 0, r.Preserved)
	require.Equal(t, 0, r.Resolved)
	require.Equal(t, 0, r.ResolvedByStrategy)
}

func cell(hours float64, sourceID int) domain.DraftCell {
	return domain.DraftCell{Hours: hours, SourceEntryID: sourceID}
}

// classifyCellTC drives a single (rowKey, weekday) classification through
// classifyCell and asserts on the merged cell + outcome counter.
type classifyCellTC struct {
	name           string
	pulled         *domain.DraftCell // nil = absent
	local          *domain.DraftCell
	remote         *domain.DraftCell
	strategy       Strategy
	wantOutcome    cellOutcome
	wantMergeCell  *domain.DraftCell
	wantConflict   bool
}

func runClassifyCellTCs(t *testing.T, tcs []classifyCellTC) {
	t.Helper()
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			res := classifyCell(tc.pulled, tc.local, tc.remote, tc.strategy)
			require.Equal(t, tc.wantOutcome, res.outcome, "outcome mismatch")
			require.Equal(t, tc.wantConflict, res.conflict != nil, "conflict presence mismatch")
			if tc.wantMergeCell == nil {
				require.Nil(t, res.merged, "expected no merged cell")
			} else {
				require.NotNil(t, res.merged, "expected a merged cell")
				require.Equal(t, tc.wantMergeCell.Hours, res.merged.Hours, "merged hours")
				require.Equal(t, tc.wantMergeCell.SourceEntryID, res.merged.SourceEntryID, "merged sourceID")
			}
		})
	}
}

func TestClassifyCell_HappyPaths(t *testing.T) {
	c := cell(4, 100)
	c2 := cell(6, 100)
	cAdd := cell(3, 0)
	tcs := []classifyCellTC{
		{
			name:          "untouched: same on all three views",
			pulled:        &c, local: &c, remote: &c,
			strategy:      StrategyAbort,
			wantOutcome:   outcomeUntouched,
			wantMergeCell: &c,
		},
		{
			name:          "adopt remote: local unchanged, remote changed",
			pulled:        &c, local: &c, remote: &c2,
			strategy:      StrategyAbort,
			wantOutcome:   outcomeAdopted,
			wantMergeCell: &c2,
		},
		{
			name:          "preserve local: local changed, remote unchanged",
			pulled:        &c, local: &c2, remote: &c,
			strategy:      StrategyAbort,
			wantOutcome:   outcomePreserved,
			wantMergeCell: &c2,
		},
		{
			name:          "converged: both changed to same value",
			pulled:        &c, local: &c2, remote: &c2,
			strategy:      StrategyAbort,
			wantOutcome:   outcomeResolved,
			wantMergeCell: &c2,
		},
		{
			name:          "remote-only added (didn't exist locally)",
			pulled:        nil, local: nil, remote: &cAdd,
			strategy:      StrategyAbort,
			wantOutcome:   outcomeAdopted,
			wantMergeCell: &cAdd,
		},
		{
			name:          "local-only added (still not on remote)",
			pulled:        nil, local: &cAdd, remote: nil,
			strategy:      StrategyAbort,
			wantOutcome:   outcomePreserved,
			wantMergeCell: &cAdd,
		},
	}
	runClassifyCellTCs(t, tcs)
}
