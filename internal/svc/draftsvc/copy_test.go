package draftsvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

func TestService_Copy_SameDateAlternate(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	src := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
		Notes: "primary",
		Rows:  []domain.DraftRow{{ID: "row-01", Cells: []domain.DraftCell{{Day: time.Monday, Hours: 4}}}},
	}
	if err := s.store.Save(src); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SavePulledSnapshot(src); err != nil {
		t.Fatal(err)
	}

	dst, err := s.Copy("work", week, "default", "work", week, "pristine")
	if err != nil {
		t.Fatal(err)
	}
	if dst.Name != "pristine" || !dst.WeekStart.Equal(week) {
		t.Errorf("dst identity wrong: %+v", dst)
	}
	if dst.Notes != "primary" {
		t.Errorf("dst.Notes = %q, want %q", dst.Notes, "primary")
	}
	if !s.store.Exists("work", week, "pristine") {
		t.Errorf("dst not saved")
	}
	if _, err := s.store.LoadPulledSnapshot("work", week, "pristine"); err != nil {
		t.Errorf("dst pulled snapshot missing: %v", err)
	}
}

func TestService_Copy_CrossWeek_DropsPulledSnapshot(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	srcWeek := time.Date(2026, 4, 26, 0, 0, 0, 0, domain.EasternTZ)
	dstWeek := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	src := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: srcWeek,
		Rows: []domain.DraftRow{{ID: "row-01", Cells: []domain.DraftCell{{Day: time.Monday, Hours: 4, SourceEntryID: 100}}}},
	}
	if err := s.store.Save(src); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SavePulledSnapshot(src); err != nil {
		t.Fatal(err)
	}

	dst, err := s.Copy("work", srcWeek, "default", "work", dstWeek, "default")
	if err != nil {
		t.Fatal(err)
	}
	if !dst.WeekStart.Equal(dstWeek) {
		t.Errorf("dst week wrong")
	}
	if _, err := s.store.LoadPulledSnapshot("work", dstWeek, "default"); err == nil {
		t.Errorf("dst should NOT have a pulled snapshot for cross-week copy")
	}
	if dst.Rows[0].Cells[0].SourceEntryID != 0 {
		t.Errorf("cross-week copy should clear sourceEntryIDs, got %d",
			dst.Rows[0].Cells[0].SourceEntryID)
	}
}

func TestService_Copy_RefusesCollision(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	src := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}
	dst := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "pristine", WeekStart: week}
	if err := s.store.Save(src); err != nil {
		t.Fatal(err)
	}
	if err := s.store.Save(dst); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Copy("work", week, "default", "work", week, "pristine"); err == nil {
		t.Errorf("Copy should refuse on dst collision")
	}
}
