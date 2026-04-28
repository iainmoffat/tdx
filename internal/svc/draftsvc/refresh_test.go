package draftsvc

import (
	"testing"
	"time"

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

func TestClassifyCell_AbortConflicts(t *testing.T) {
	pulled := cell(4, 100)
	localEdit := cell(6, 100)
	remoteEdit := cell(8, 100)
	cleared := cell(0, 100) // hours=0 but sourceEntryID kept = "delete on push"
	addLocal := cell(3, 0)
	addRemoteSame := cell(3, 555) // remote added same target/day independently
	addRemoteDifferent := cell(5, 555)

	tcs := []classifyCellTC{
		{
			name:         "both changed, different values -> conflict",
			pulled:       &pulled, local: &localEdit, remote: &remoteEdit,
			strategy:     StrategyAbort,
			wantOutcome:  outcomeNone, // abort reports conflict, no merged cell
			wantConflict: true,
		},
		{
			name:         "local edited, remote deleted -> conflict",
			pulled:       &pulled, local: &localEdit, remote: nil,
			strategy:     StrategyAbort,
			wantOutcome:  outcomeNone,
			wantConflict: true,
		},
		{
			name:         "local cleared, remote modified -> conflict",
			pulled:       &pulled, local: &cleared, remote: &remoteEdit,
			strategy:     StrategyAbort,
			wantOutcome:  outcomeNone,
			wantConflict: true,
		},
		{
			name:         "both added different rows-on-same-key -> conflict",
			pulled:       nil, local: &addLocal, remote: &addRemoteDifferent,
			strategy:     StrategyAbort,
			wantOutcome:  outcomeNone,
			wantConflict: true,
		},
		{
			name:          "both added same hours -> resolved (converged add)",
			pulled:        nil, local: &addLocal, remote: &addRemoteSame,
			strategy:      StrategyAbort,
			wantOutcome:   outcomeResolved,
			wantMergeCell: &addRemoteSame, // adopt remote's sourceEntryID
		},
	}
	runClassifyCellTCs(t, tcs)
}

func TestClassifyCell_StrategyOursResolves(t *testing.T) {
	pulled := cell(4, 100)
	localEdit := cell(6, 100)
	remoteEdit := cell(8, 100)

	res := classifyCell(&pulled, &localEdit, &remoteEdit, StrategyOurs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.Nil(t, res.conflict)
	require.NotNil(t, res.merged)
	require.Equal(t, 6.0, res.merged.Hours, "ours keeps local")
}

func TestClassifyCell_StrategyTheirsResolves(t *testing.T) {
	pulled := cell(4, 100)
	localEdit := cell(6, 100)
	remoteEdit := cell(8, 100)

	res := classifyCell(&pulled, &localEdit, &remoteEdit, StrategyTheirs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.Nil(t, res.conflict)
	require.NotNil(t, res.merged)
	require.Equal(t, 8.0, res.merged.Hours, "theirs takes remote")
}

func TestClassifyCell_StrategyOursOnEditVsDelete(t *testing.T) {
	pulled := cell(4, 100)
	localEdit := cell(6, 100)

	res := classifyCell(&pulled, &localEdit, nil, StrategyOurs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.NotNil(t, res.merged)
	require.Equal(t, 6.0, res.merged.Hours)
}

func TestClassifyCell_StrategyTheirsOnEditVsDelete(t *testing.T) {
	pulled := cell(4, 100)
	localEdit := cell(6, 100)

	res := classifyCell(&pulled, &localEdit, nil, StrategyTheirs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.Nil(t, res.merged, "theirs accepts the remote delete")
}

func TestClassifyCell_StrategyOursOnClearedVsModified(t *testing.T) {
	pulled := cell(4, 100)
	cleared := cell(0, 100)
	remoteEdit := cell(8, 100)

	res := classifyCell(&pulled, &cleared, &remoteEdit, StrategyOurs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.NotNil(t, res.merged)
	require.Equal(t, 0.0, res.merged.Hours)
	require.Equal(t, 100, res.merged.SourceEntryID, "still flagged for delete")
}

func TestClassifyCell_StrategyTheirsOnClearedVsModified(t *testing.T) {
	pulled := cell(4, 100)
	cleared := cell(0, 100)
	remoteEdit := cell(8, 100)

	res := classifyCell(&pulled, &cleared, &remoteEdit, StrategyTheirs)
	require.Equal(t, outcomeResolvedByStrategy, res.outcome)
	require.NotNil(t, res.merged)
	require.Equal(t, 8.0, res.merged.Hours, "theirs takes the remote modification")
}

func TestClassifyCell_StaleSource(t *testing.T) {
	pulled := cell(4, 100)
	local := cell(4, 100) // unchanged from pull
	res := classifyCell(&pulled, &local, nil, StrategyAbort)
	require.Equal(t, outcomeAdopted, res.outcome,
		"local unchanged + remote deleted = adopt remote (clear sourceID, mark as added)")
	require.NotNil(t, res.merged)
	require.Equal(t, 4.0, res.merged.Hours, "hours preserved")
	require.Equal(t, 0, res.merged.SourceEntryID, "sourceID cleared (re-add on next push)")
}

func TestClassifyCell_ClearedAndDeleted_DropsOut(t *testing.T) {
	pulled := cell(4, 100)
	cleared := cell(0, 100)
	res := classifyCell(&pulled, &cleared, nil, StrategyAbort)
	require.Equal(t, outcomeDropped, res.outcome,
		"local intent (delete) and remote reality (already deleted) match -> drop")
	require.Nil(t, res.merged)
	require.Nil(t, res.conflict)
}

func TestClassifyRow_MixedOutcomesPerRow(t *testing.T) {
	row := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 6, SourceEntryID: 100},  // edited locally
			{Day: time.Tuesday, Hours: 4, SourceEntryID: 101}, // unchanged
		},
	}
	pulled := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 4, SourceEntryID: 100},
			{Day: time.Tuesday, Hours: 4, SourceEntryID: 101},
		},
	}
	remote := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 4, SourceEntryID: 100},   // unchanged
			{Day: time.Tuesday, Hours: 8, SourceEntryID: 101},  // edited remotely
			{Day: time.Wednesday, Hours: 3, SourceEntryID: 102}, // added remotely
		},
	}

	merged, counts, conflicts := classifyRow("row-01", &pulled, &row, &remote, StrategyAbort)
	require.Empty(t, conflicts)
	require.Equal(t, 2, counts.adopted, "Tue (remote change taken) + Wed (remote-only add) both adopted")
	require.Equal(t, 1, counts.preserved, "Mon should be preserved (local edit)")
	require.Equal(t, 0, counts.resolved)
	// Untouched cells aren't counted in the result struct, but they still
	// flow through merged.
	require.Len(t, merged, 3, "Mon (kept) + Tue (adopted) + Wed (added remote)")
}

func TestClassifyRow_ConflictCarriesRowAndDay(t *testing.T) {
	row := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 6, SourceEntryID: 100},
		},
	}
	pulled := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 4, SourceEntryID: 100},
		},
	}
	remote := domain.DraftRow{
		ID: "row-01",
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 8, SourceEntryID: 100},
		},
	}

	_, _, conflicts := classifyRow("row-01", &pulled, &row, &remote, StrategyAbort)
	require.Len(t, conflicts, 1)
	require.Equal(t, "row-01", conflicts[0].RowID)
	require.Equal(t, "Monday", conflicts[0].Day)
	require.Equal(t, "updated to 6.0h", conflicts[0].LocalDescription)
	require.Equal(t, "updated to 8.0h", conflicts[0].RemoteDescription)
}
