package draftsvc

import (
	"context"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
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

// makeRow is a test helper for building DraftRow values with a target/type
// signature that can be matched across views by rowKey().
func makeRow(id string, kind domain.TargetKind, itemID, typeID int, cells ...domain.DraftCell) domain.DraftRow {
	return domain.DraftRow{
		ID:       id,
		Target:   domain.Target{Kind: kind, ItemID: itemID},
		TimeType: domain.TimeType{ID: typeID},
		Cells:    cells,
	}
}

func TestClassify_AdoptsNewRemoteRow(t *testing.T) {
	pulled := domain.WeekDraft{Profile: "p", Name: "default"}
	local := domain.WeekDraft{Profile: "p", Name: "default"}
	remote := domain.WeekDraft{
		Profile: "p", Name: "default",
		Rows: []domain.DraftRow{
			makeRow("row-01", domain.TargetTicket, 555, 17,
				domain.DraftCell{Day: time.Monday, Hours: 4, SourceEntryID: 900}),
		},
	}

	res := classify(pulled, local, remote, StrategyAbort)
	require.False(t, res.aborted)
	require.Empty(t, res.conflicts)
	require.Equal(t, 1, res.counts.adopted)
	require.Len(t, res.rows, 1, "new remote row joins merged set")
	require.Len(t, res.rows[0].Cells, 1)
}

func TestClassify_KeepsLocalOnlyRow(t *testing.T) {
	local := domain.WeekDraft{
		Profile: "p", Name: "default",
		Rows: []domain.DraftRow{
			makeRow("row-localnew", domain.TargetTicket, 777, 19,
				domain.DraftCell{Day: time.Tuesday, Hours: 2}),
		},
	}
	pulled := domain.WeekDraft{Profile: "p", Name: "default"}
	remote := domain.WeekDraft{Profile: "p", Name: "default"}

	res := classify(pulled, local, remote, StrategyAbort)
	require.False(t, res.aborted)
	require.Empty(t, res.conflicts)
	require.Equal(t, 1, res.counts.preserved)
	require.Len(t, res.rows, 1)
	require.Equal(t, "row-localnew", res.rows[0].ID, "local rowID preserved")
}

func TestClassify_AbortsOnConflict(t *testing.T) {
	row := makeRow("row-01", domain.TargetTicket, 555, 17)
	row.Cells = []domain.DraftCell{{Day: time.Monday, Hours: 6, SourceEntryID: 900}}
	rowPulled := makeRow("row-01", domain.TargetTicket, 555, 17)
	rowPulled.Cells = []domain.DraftCell{{Day: time.Monday, Hours: 4, SourceEntryID: 900}}
	rowRemote := makeRow("row-01", domain.TargetTicket, 555, 17)
	rowRemote.Cells = []domain.DraftCell{{Day: time.Monday, Hours: 8, SourceEntryID: 900}}

	res := classify(
		domain.WeekDraft{Rows: []domain.DraftRow{rowPulled}},
		domain.WeekDraft{Rows: []domain.DraftRow{row}},
		domain.WeekDraft{Rows: []domain.DraftRow{rowRemote}},
		StrategyAbort,
	)
	require.True(t, res.aborted)
	require.Len(t, res.conflicts, 1)
	require.Equal(t, "row-01", res.conflicts[0].RowID)
	require.Empty(t, res.rows, "abort means no merged rows")
}

func TestClassify_StrategyOursCollapsesConflict(t *testing.T) {
	rowPulled := makeRow("row-01", domain.TargetTicket, 555, 17,
		domain.DraftCell{Day: time.Monday, Hours: 4, SourceEntryID: 900})
	rowLocal := makeRow("row-01", domain.TargetTicket, 555, 17,
		domain.DraftCell{Day: time.Monday, Hours: 6, SourceEntryID: 900})
	rowRemote := makeRow("row-01", domain.TargetTicket, 555, 17,
		domain.DraftCell{Day: time.Monday, Hours: 8, SourceEntryID: 900})

	res := classify(
		domain.WeekDraft{Rows: []domain.DraftRow{rowPulled}},
		domain.WeekDraft{Rows: []domain.DraftRow{rowLocal}},
		domain.WeekDraft{Rows: []domain.DraftRow{rowRemote}},
		StrategyOurs,
	)
	require.False(t, res.aborted)
	require.Empty(t, res.conflicts)
	require.Equal(t, 1, res.counts.resolvedByStrategy)
	require.Len(t, res.rows, 1)
	require.Equal(t, 6.0, res.rows[0].Cells[0].Hours, "ours kept local 6.0h")
}

func TestService_Refresh_AbortPath_NoMutation(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	mock := &mockTimeWriter{
		weekRpt: domain.WeekReport{
			WeekRef: domain.WeekRef{StartDate: weekStartTuesday0501()},
			Entries: []domain.TimeEntry{
				{
					ID: 900, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, domain.EasternTZ),
					Minutes: 480, // 8h on remote
					Target:  domain.Target{Kind: domain.TargetTicket, ItemID: 555},
					TimeType: domain.TimeType{ID: 17},
				},
			},
		},
	}
	svc := newServiceWithTimeWriter(paths, mock)

	// Set up: pull (4h), then locally edit to 6h.
	weekStart := weekStartTuesday0501()
	pulledReport := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: weekStart},
		Entries: []domain.TimeEntry{
			{
				ID: 900, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, domain.EasternTZ),
				Minutes: 240, // 4h originally pulled
				Target:  domain.Target{Kind: domain.TargetTicket, ItemID: 555},
				TimeType: domain.TimeType{ID: 17},
			},
		},
	}
	mock.weekRpt = pulledReport
	_, err := svc.Pull(context.Background(), "p", weekStart, "default", false)
	require.NoError(t, err)

	// Edit local to 6h.
	d, err := svc.Store().Load("p", weekStart, "default")
	require.NoError(t, err)
	d.Rows[0].Cells[0].Hours = 6
	require.NoError(t, svc.Store().Save(d))

	// Now remote bumps to 8h; refresh under abort.
	mock.weekRpt.Entries[0].Minutes = 480

	res, err := svc.Refresh(context.Background(), "p", weekStart, "default", StrategyAbort)
	require.NoError(t, err)
	require.True(t, res.Aborted)
	require.Len(t, res.Conflicts, 1)
	require.Equal(t, StrategyAbort, res.Strategy)

	// Disk verification: local draft still has 6h.
	post, err := svc.Store().Load("p", weekStart, "default")
	require.NoError(t, err)
	require.Equal(t, 6.0, post.Rows[0].Cells[0].Hours, "abort must not mutate disk")
}

// weekStartTuesday0501 returns the Sunday containing 2026-05-04 (a Tuesday)
// in EasternTZ — i.e. 2026-05-03.
func weekStartTuesday0501() time.Time {
	return time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
}

func TestService_Refresh_SuccessPath_WritesMergedDraftAndWatermark(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	mock := &mockTimeWriter{}
	svc := newServiceWithTimeWriter(paths, mock)

	weekStart := weekStartTuesday0501()
	// Initial pull: Mon=4h, Tue=4h.
	mock.weekRpt = domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: weekStart},
		Entries: []domain.TimeEntry{
			{ID: 900, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, domain.EasternTZ), Minutes: 240,
				Target: domain.Target{Kind: domain.TargetTicket, ItemID: 555},
				TimeType: domain.TimeType{ID: 17}},
			{ID: 901, Date: time.Date(2026, 5, 5, 0, 0, 0, 0, domain.EasternTZ), Minutes: 240,
				Target: domain.Target{Kind: domain.TargetTicket, ItemID: 555},
				TimeType: domain.TimeType{ID: 17}},
		},
	}
	_, err := svc.Pull(context.Background(), "p", weekStart, "default", false)
	require.NoError(t, err)

	// User edits Mon to 6h.
	d, _ := svc.Store().Load("p", weekStart, "default")
	for i := range d.Rows[0].Cells {
		if d.Rows[0].Cells[i].Day == time.Monday {
			d.Rows[0].Cells[i].Hours = 6
		}
	}
	require.NoError(t, svc.Store().Save(d))

	// Remote independently bumps Tue to 8h (no conflict, just adopt).
	mock.weekRpt.Entries[1].Minutes = 480

	res, err := svc.Refresh(context.Background(), "p", weekStart, "default", StrategyAbort)
	require.NoError(t, err)
	require.False(t, res.Aborted)
	require.Equal(t, 1, res.Adopted, "Tue adopted from remote")
	require.Equal(t, 1, res.Preserved, "Mon edit preserved")

	// Local draft now has Mon=6 + Tue=8.
	post, _ := svc.Store().Load("p", weekStart, "default")
	cells := map[time.Weekday]float64{}
	for _, c := range post.Rows[0].Cells {
		cells[c.Day] = c.Hours
	}
	require.Equal(t, 6.0, cells[time.Monday])
	require.Equal(t, 8.0, cells[time.Tuesday])

	// Watermark now matches the post-refresh remote: a second refresh is a no-op.
	res2, err := svc.Refresh(context.Background(), "p", weekStart, "default", StrategyAbort)
	require.NoError(t, err)
	require.False(t, res2.Aborted)
	require.Equal(t, 0, res2.Adopted, "second refresh should be a no-op")
	require.Equal(t, 1, res2.Preserved, "Mon edit still local-only relative to new watermark")
}

func TestService_Refresh_TakesPreRefreshSnapshot(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	mock := &mockTimeWriter{}
	svc := newServiceWithTimeWriter(paths, mock)

	weekStart := weekStartTuesday0501()
	mock.weekRpt = domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: weekStart},
		Entries: []domain.TimeEntry{
			{ID: 900, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, domain.EasternTZ), Minutes: 240,
				Target: domain.Target{Kind: domain.TargetTicket, ItemID: 555},
				TimeType: domain.TimeType{ID: 17}},
		},
	}
	_, err := svc.Pull(context.Background(), "p", weekStart, "default", false)
	require.NoError(t, err)

	// Edit local to 6h, remote bumps to 8h, refresh ours.
	d, _ := svc.Store().Load("p", weekStart, "default")
	d.Rows[0].Cells[0].Hours = 6
	require.NoError(t, svc.Store().Save(d))
	mock.weekRpt.Entries[0].Minutes = 480

	_, err = svc.Refresh(context.Background(), "p", weekStart, "default", StrategyOurs)
	require.NoError(t, err)

	snaps, err := svc.Snapshots().List("p", weekStart, "default")
	require.NoError(t, err)
	var preRefresh []SnapshotInfo
	for _, s := range snaps {
		if s.Op == OpPreRefresh {
			preRefresh = append(preRefresh, s)
		}
	}
	require.Len(t, preRefresh, 1, "exactly one pre-refresh snapshot")
}

func TestService_Refresh_AbortPath_NoSnapshot(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	mock := &mockTimeWriter{}
	svc := newServiceWithTimeWriter(paths, mock)

	weekStart := weekStartTuesday0501()
	mock.weekRpt = domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: weekStart},
		Entries: []domain.TimeEntry{
			{ID: 900, Date: time.Date(2026, 5, 4, 0, 0, 0, 0, domain.EasternTZ), Minutes: 240,
				Target: domain.Target{Kind: domain.TargetTicket, ItemID: 555},
				TimeType: domain.TimeType{ID: 17}},
		},
	}
	_, err := svc.Pull(context.Background(), "p", weekStart, "default", false)
	require.NoError(t, err)

	d, _ := svc.Store().Load("p", weekStart, "default")
	d.Rows[0].Cells[0].Hours = 6
	require.NoError(t, svc.Store().Save(d))
	mock.weekRpt.Entries[0].Minutes = 480

	_, err = svc.Refresh(context.Background(), "p", weekStart, "default", StrategyAbort)
	require.NoError(t, err)

	snaps, err := svc.Snapshots().List("p", weekStart, "default")
	require.NoError(t, err)
	for _, s := range snaps {
		require.NotEqual(t, OpPreRefresh, s.Op, "abort must not snapshot")
	}
}
