package draftsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestPull_ResolvesDefaultTimeTypeForPlaceholders(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	mock := &mockTimeWriter{
		weekRpt: domain.WeekReport{
			WeekRef: domain.WeekRef{StartDate: week},
			Entries: []domain.TimeEntry{
				// Real entry — already has TimeType.
				{ID: 100, Date: week.AddDate(0, 0, 1), Minutes: 240,
					Target:   domain.Target{Kind: domain.TargetProjectTask, ProjectID: 52, ItemID: 2075, TaskID: 2076},
					TimeType: domain.TimeType{ID: 5, Name: "Standard Activities"}},
				// Placeholder with TypeID=0 — should be resolved.
				{ID: 0, Minutes: 0,
					Target:   domain.Target{Kind: domain.TargetProjectTask, ProjectID: 259, ItemID: 1292, TaskID: 4938},
					TimeType: domain.TimeType{ID: 0}},
			},
		},
		typesFor: map[string][]domain.TimeType{
			"projectTask:0:259:1292:4938": {
				{ID: 1, Name: "Project"},
				{ID: 7, Name: "Training"},
			},
		},
	}
	svc := newServiceWithTimeWriter(paths, mock)

	draft, err := svc.Pull(context.Background(), "p", week, "default", false)
	require.NoError(t, err)
	require.Len(t, draft.Rows, 2)

	// Find the placeholder row by target.
	var placeholder *domain.DraftRow
	for i := range draft.Rows {
		if draft.Rows[i].Target.TaskID == 4938 {
			placeholder = &draft.Rows[i]
		}
	}
	require.NotNil(t, placeholder)
	require.Equal(t, 1, placeholder.TimeType.ID, "resolution picks the first TimeType")
	require.Equal(t, "Project", placeholder.TimeType.Name)
}

func TestPull_KeepsTypeIDZeroIfLookupFails(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	mock := &mockTimeWriter{
		weekRpt: domain.WeekReport{
			WeekRef: domain.WeekRef{StartDate: week},
			Entries: []domain.TimeEntry{
				{ID: 0, Minutes: 0,
					Target:   domain.Target{Kind: domain.TargetProjectTask, ProjectID: 100, ItemID: 200, TaskID: 300},
					TimeType: domain.TimeType{ID: 0}},
			},
		},
		typesErr: errors.New("simulated TD failure"),
	}
	svc := newServiceWithTimeWriter(paths, mock)

	draft, err := svc.Pull(context.Background(), "p", week, "default", false)
	require.NoError(t, err)
	require.Len(t, draft.Rows, 1)
	require.Equal(t, 0, draft.Rows[0].TimeType.ID, "lookup failure leaves TypeID=0; reconcile guard catches at push")
}

func TestPull_DedupesPlaceholderCollidingWithRealEntry(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	target := domain.Target{Kind: domain.TargetProjectTask, ProjectID: 52, ItemID: 2075, TaskID: 2076}
	mock := &mockTimeWriter{
		weekRpt: domain.WeekReport{
			WeekRef: domain.WeekRef{StartDate: week},
			Entries: []domain.TimeEntry{
				// Real entry on (target, TypeID=5).
				{ID: 100, Date: week.AddDate(0, 0, 1), Minutes: 240,
					Target:   target,
					TimeType: domain.TimeType{ID: 5, Name: "Standard Activities"}},
				// Placeholder on the same target with TypeID=0.
				{ID: 0, Minutes: 0,
					Target:   target,
					TimeType: domain.TimeType{ID: 0}},
			},
		},
		// Resolution picks ID=5 (same as the real entry) → collision.
		typesFor: map[string][]domain.TimeType{
			"projectTask:0:52:2075:2076": {
				{ID: 5, Name: "Standard Activities"},
			},
		},
	}
	svc := newServiceWithTimeWriter(paths, mock)

	draft, err := svc.Pull(context.Background(), "p", week, "default", false)
	require.NoError(t, err)
	require.Len(t, draft.Rows, 1, "placeholder dedups against the real-entry row")
	require.Len(t, draft.Rows[0].Cells, 1, "real entry's cell preserved")
	require.Equal(t, 5, draft.Rows[0].TimeType.ID)
}

func TestPull_CachesTimeTypesPerTarget(t *testing.T) {
	tmp := t.TempDir()
	paths := config.Paths{Root: tmp}
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	target := domain.Target{Kind: domain.TargetProjectTask, ProjectID: 52, ItemID: 2075, TaskID: 2076}
	mock := &mockTimeWriter{
		weekRpt: domain.WeekReport{
			WeekRef: domain.WeekRef{StartDate: week},
			Entries: []domain.TimeEntry{
				// Two placeholders on the same target. (Wouldn't happen in
				// practice from one report, but tests the cache.)
				{ID: 0, Minutes: 0, Target: target, TimeType: domain.TimeType{ID: 0}, Billable: false},
				{ID: 0, Minutes: 0, Target: target, TimeType: domain.TimeType{ID: 0}, Billable: true},
			},
		},
		typesFor: map[string][]domain.TimeType{
			"projectTask:0:52:2075:2076": {{ID: 5, Name: "Standard Activities"}},
		},
	}
	svc := newServiceWithTimeWriter(paths, mock)

	_, err := svc.Pull(context.Background(), "p", week, "default", false)
	require.NoError(t, err)
	require.Equal(t, 1, mock.typesCalls, "TimeTypesForTarget should be cached per target")
}
