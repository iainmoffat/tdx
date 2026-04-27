package draftsvc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

type mockTimeWriter struct {
	creates []domain.EntryInput
	updates []struct {
		ID    int
		Patch domain.EntryUpdate
	}
	deletes []int
	failOn  string // "create" | "update" | "delete"
	weekRpt domain.WeekReport
	locked  []domain.LockedDay
}

func (m *mockTimeWriter) AddEntry(_ context.Context, _ string, e domain.EntryInput) (domain.TimeEntry, error) {
	if m.failOn == "create" {
		return domain.TimeEntry{}, fmt.Errorf("simulated create failure")
	}
	m.creates = append(m.creates, e)
	return domain.TimeEntry{ID: 1000 + len(m.creates)}, nil
}

func (m *mockTimeWriter) UpdateEntry(_ context.Context, _ string, id int, p domain.EntryUpdate) (domain.TimeEntry, error) {
	if m.failOn == "update" {
		return domain.TimeEntry{}, fmt.Errorf("simulated update failure")
	}
	m.updates = append(m.updates, struct {
		ID    int
		Patch domain.EntryUpdate
	}{id, p})
	return domain.TimeEntry{ID: id}, nil
}

func (m *mockTimeWriter) DeleteEntry(_ context.Context, _ string, id int) error {
	if m.failOn == "delete" {
		return fmt.Errorf("simulated delete failure")
	}
	m.deletes = append(m.deletes, id)
	return nil
}

func (m *mockTimeWriter) GetWeekReport(_ context.Context, _ string, _ time.Time) (domain.WeekReport, error) {
	return m.weekRpt, nil
}

func (m *mockTimeWriter) GetLockedDays(_ context.Context, _ string, _, _ time.Time) ([]domain.LockedDay, error) {
	return m.locked, nil
}

func TestApply_AllowDeletesGate(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	paths := config.Paths{Root: t.TempDir()}

	target := domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123}
	timeType := domain.TimeType{ID: 7, Name: "Work"}

	// Draft with one cleared pulled cell (delete on push).
	draft := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
		Provenance: domain.DraftProvenance{
			Kind: domain.ProvenancePulled, RemoteFingerprint: "fp1",
			RemoteStatus: domain.ReportOpen,
		},
		Rows: []domain.DraftRow{{
			ID:       "row-01",
			Target:   target, TimeType: timeType, Billable: true,
			Cells: []domain.DraftCell{{Day: time.Monday, Hours: 0, SourceEntryID: 98731}},
		}},
	}
	store := NewStore(paths)
	if err := store.Save(draft); err != nil {
		t.Fatal(err)
	}

	// Pulled-snapshot sibling so dirty-detection sees the cell as edited.
	pulled := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
		Rows: []domain.DraftRow{{
			ID:    "row-01",
			Cells: []domain.DraftCell{{Day: time.Monday, Hours: 8.0, SourceEntryID: 98731}},
		}},
	}
	if err := store.SavePulledSnapshot(pulled); err != nil {
		t.Fatal(err)
	}

	// Mock remote: contains the entry that the draft says was pulled.
	mw := &mockTimeWriter{
		weekRpt: domain.WeekReport{
			WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
			Status:  domain.ReportOpen,
			Entries: []domain.TimeEntry{
				{ID: 98731, Date: week.AddDate(0, 0, 1), Minutes: 480,
					Target: target, TimeType: timeType, Billable: true},
			},
		},
	}
	s := newServiceWithTimeWriter(paths, mw)

	// Preview to capture diff hash.
	_, diff, err := s.Reconcile(context.Background(), "work", week, "default", "user-1")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// allowDeletes=false -> refuse.
	_, err = s.Apply(context.Background(), "work", week, "default", diff.DiffHash, false, "user-1")
	if err == nil {
		t.Fatal("expected error when allowDeletes=false with delete actions")
	}
	if len(mw.deletes) != 0 {
		t.Errorf("delete attempted despite refusal: %v", mw.deletes)
	}

	// allowDeletes=true -> success.
	res, err := s.Apply(context.Background(), "work", week, "default", diff.DiffHash, true, "user-1")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Deleted != 1 {
		t.Errorf("Deleted = %d, want 1", res.Deleted)
	}
	if len(mw.deletes) != 1 || mw.deletes[0] != 98731 {
		t.Errorf("expected delete of 98731, got %v", mw.deletes)
	}
}

func TestApply_HashMismatchRefuses(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	paths := config.Paths{Root: t.TempDir()}

	draft := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
		Provenance: domain.DraftProvenance{Kind: domain.ProvenancePulled, RemoteFingerprint: "fp1"},
		Rows: []domain.DraftRow{{
			ID:       "row-01",
			Target:   domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 1},
			TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
			Cells: []domain.DraftCell{{Day: time.Monday, Hours: 4.0}},
		}},
	}
	store := NewStore(paths)
	if err := store.Save(draft); err != nil {
		t.Fatal(err)
	}

	mw := &mockTimeWriter{
		weekRpt: domain.WeekReport{
			WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
			Status:  domain.ReportOpen,
		},
	}
	s := newServiceWithTimeWriter(paths, mw)

	_, err := s.Apply(context.Background(), "work", week, "default", "wrong-hash", false, "user-1")
	if err == nil {
		t.Fatal("expected hash mismatch error")
	}
	if len(mw.creates)+len(mw.updates)+len(mw.deletes) > 0 {
		t.Error("writes attempted despite hash mismatch")
	}
}
