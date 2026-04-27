package draftsvc

import (
	"context"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

func TestService_Reset_DiscardsLocalAndRePulls(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	target := domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 1}
	timeType := domain.TimeType{ID: 7, Name: "Work"}

	mw := &mockTimeWriter{
		weekRpt: domain.WeekReport{
			WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
			Status:  domain.ReportOpen,
			Entries: []domain.TimeEntry{
				{ID: 100, Date: week.AddDate(0, 0, 1), Minutes: 480,
					Target: target, TimeType: timeType, Billable: true},
			},
		},
	}
	s := newServiceWithTimeWriter(paths, mw)

	edited := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
		Notes: "local edits",
		Rows: []domain.DraftRow{{ID: "row-99", Cells: []domain.DraftCell{{Day: time.Friday, Hours: 99}}}},
	}
	if err := s.store.Save(edited); err != nil {
		t.Fatal(err)
	}

	if err := s.Reset(context.Background(), "work", week, "default"); err != nil {
		t.Fatal(err)
	}

	fresh, err := s.store.Load("work", week, "default")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Notes == "local edits" {
		t.Errorf("Reset did not discard notes")
	}
	if len(fresh.Rows) != 1 || fresh.Rows[0].Cells[0].SourceEntryID != 100 {
		t.Errorf("Reset did not produce fresh-pull rows: %+v", fresh.Rows)
	}

	list, err := s.snapshots.List("work", week, "default")
	if err != nil {
		t.Fatal(err)
	}
	var hasPreReset bool
	for _, sn := range list {
		if sn.Op == OpPreReset {
			hasPreReset = true
		}
	}
	if !hasPreReset {
		t.Errorf("no pre-reset snapshot taken")
	}
}
