package draftsvc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

// resolveHarness writes a conflicted draft to disk and returns the
// service + draft locator (profile, weekStart, name).
func resolveHarness(t *testing.T, draft domain.WeekDraft) (*Service, string, time.Time, string) {
	t.Helper()
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	svc := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	require.NoError(t, svc.store.Save(draft))
	return svc, draft.Profile, draft.WeekStart, draft.Name
}

func conflictedDraft() domain.WeekDraft {
	weekStart := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	return domain.WeekDraft{
		SchemaVersion: 1,
		Profile:       "default",
		Name:          "default",
		WeekStart:     weekStart,
		Rows: []domain.DraftRow{{
			ID: "row-01",
			Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 555},
			TimeType: domain.TimeType{ID: 7},
			Billable: true,
			Cells: []domain.DraftCell{
				{Day: time.Monday, Hours: 6, SourceEntryID: 900,
					Conflict: &domain.DraftConflictAlt{Hours: 8, SourceEntryID: 900, PulledHours: 4}},
				{Day: time.Tuesday, Hours: 6, SourceEntryID: 901,
					Conflict: &domain.DraftConflictAlt{Hours: 0, SourceEntryID: 0, PulledHours: 4}}, // remote deleted
				{Day: time.Wednesday, Hours: 5, SourceEntryID: 902}, // not conflicted
			},
		}},
		Provenance: domain.DraftProvenance{Kind: domain.ProvenancePulled},
		ModifiedAt: time.Now().UTC(),
	}
}

func TestListConflicts_FindsAll(t *testing.T) {
	svc, profile, weekStart, name := resolveHarness(t, conflictedDraft())
	conflicts, err := svc.ListConflicts(profile, weekStart, name)
	require.NoError(t, err)
	require.Len(t, conflicts, 2)
	require.Equal(t, "row-01", conflicts[0].RowID)
	require.Equal(t, time.Monday, conflicts[0].Day)
	require.Equal(t, 6.0, conflicts[0].LocalHours)
	require.Equal(t, 8.0, conflicts[0].RemoteHours)
	require.False(t, conflicts[0].RemoteDeletes)
	require.Equal(t, time.Tuesday, conflicts[1].Day)
	require.True(t, conflicts[1].RemoteDeletes, "remote deleted should set RemoteDeletes")
}

func TestResolve_PickAllLocal_ClearsAllConflicts(t *testing.T) {
	svc, profile, weekStart, name := resolveHarness(t, conflictedDraft())
	res, err := svc.Resolve(profile, weekStart, name, true, false, nil, false)
	require.NoError(t, err)
	require.Equal(t, 2, res.PicksApplied)
	require.Equal(t, 2, res.PickedLocal)
	require.Equal(t, 0, res.PickedRemote)
	require.Equal(t, 0, res.ConflictsRemaining)

	loaded, err := svc.store.Load(profile, weekStart, name)
	require.NoError(t, err)
	for _, row := range loaded.Rows {
		for _, c := range row.Cells {
			require.Nil(t, c.Conflict, "all Conflict alts should be cleared")
		}
	}
	// Local Monday should still be 6.0 (unchanged).
	require.Equal(t, 6.0, loaded.Rows[0].Cells[0].Hours)
}

func TestResolve_PickAllRemote_RequiresYesForDeleteCells(t *testing.T) {
	svc, profile, weekStart, name := resolveHarness(t, conflictedDraft())
	_, err := svc.Resolve(profile, weekStart, name, false, true, nil, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--yes")
}

func TestResolve_PickAllRemote_WithYesAppliesAndDropsDeletedCell(t *testing.T) {
	svc, profile, weekStart, name := resolveHarness(t, conflictedDraft())
	res, err := svc.Resolve(profile, weekStart, name, false, true, nil, true)
	require.NoError(t, err)
	require.Equal(t, 2, res.PicksApplied)
	require.Equal(t, 2, res.PickedRemote)
	require.Equal(t, 1, res.DroppedDeletedCells)
	require.Equal(t, 0, res.ConflictsRemaining)

	loaded, err := svc.store.Load(profile, weekStart, name)
	require.NoError(t, err)
	require.Len(t, loaded.Rows, 1, "row should still exist (Wednesday non-conflicted cell remains)")
	// Tuesday cell should be gone (dropped); Monday now 8.0 (remote); Wednesday unchanged.
	days := map[time.Weekday]float64{}
	for _, c := range loaded.Rows[0].Cells {
		days[c.Day] = c.Hours
	}
	require.Equal(t, 8.0, days[time.Monday])
	_, hasTuesday := days[time.Tuesday]
	require.False(t, hasTuesday, "Tuesday cell should be dropped (remote deleted)")
	require.Equal(t, 5.0, days[time.Wednesday])
}

func TestResolve_PerCellPick_Local(t *testing.T) {
	svc, profile, weekStart, name := resolveHarness(t, conflictedDraft())
	picks := []Pick{{RowID: "row-01", Day: time.Monday, Choice: PickLocal}}
	res, err := svc.Resolve(profile, weekStart, name, false, false, picks, false)
	require.NoError(t, err)
	require.Equal(t, 1, res.PicksApplied)
	require.Equal(t, 1, res.ConflictsRemaining, "Tuesday still conflicted")
}

func TestResolve_PerCellPick_Remote(t *testing.T) {
	svc, profile, weekStart, name := resolveHarness(t, conflictedDraft())
	picks := []Pick{{RowID: "row-01", Day: time.Monday, Choice: PickRemote}}
	res, err := svc.Resolve(profile, weekStart, name, false, false, picks, false)
	require.NoError(t, err)
	require.Equal(t, 1, res.PicksApplied)
	loaded, err := svc.store.Load(profile, weekStart, name)
	require.NoError(t, err)
	require.Equal(t, 8.0, loaded.Rows[0].Cells[0].Hours)
	require.Nil(t, loaded.Rows[0].Cells[0].Conflict)
}

func TestResolve_NoPicks_ReturnsCount(t *testing.T) {
	svc, profile, weekStart, name := resolveHarness(t, conflictedDraft())
	res, err := svc.Resolve(profile, weekStart, name, false, false, nil, false)
	require.NoError(t, err)
	require.Equal(t, 0, res.PicksApplied)
	require.Equal(t, 2, res.ConflictsRemaining)
}

func TestResolve_AllLocalAndAllRemoteMutuallyExclusive(t *testing.T) {
	svc, profile, weekStart, name := resolveHarness(t, conflictedDraft())
	_, err := svc.Resolve(profile, weekStart, name, true, true, nil, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestParsePickChoice(t *testing.T) {
	c, err := ParsePickChoice("LOCAL")
	require.NoError(t, err)
	require.Equal(t, PickLocal, c)
	c, err = ParsePickChoice("Remote")
	require.NoError(t, err)
	require.Equal(t, PickRemote, c)
	_, err = ParsePickChoice("nope")
	require.Error(t, err)
}

func TestParseWeekday(t *testing.T) {
	d, err := ParseWeekday("MONDAY")
	require.NoError(t, err)
	require.Equal(t, time.Monday, d)
	_, err = ParseWeekday("Funday")
	require.Error(t, err)
}
