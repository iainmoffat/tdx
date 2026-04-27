package draftsvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestBuildDraftFromReport_GroupsByTargetTypeBillable(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	report := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
		UserUID: "user-1",
		Status:  domain.ReportOpen,
		Entries: []domain.TimeEntry{
			{ID: 100, Date: week.AddDate(0, 0, 1), Minutes: 480,
				Target:   domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
				TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
				Description: "morning"},
			{ID: 101, Date: week.AddDate(0, 0, 2), Minutes: 480,
				Target:   domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
				TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
				Description: "afternoon"},
			{ID: 102, Date: week.AddDate(0, 0, 5), Minutes: 240,
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 456},
				TimeType: domain.TimeType{ID: 9, Name: "Planning"}, Billable: false},
		},
	}

	draft := buildDraftFromReport("work", "default", report)

	if got := len(draft.Rows); got != 2 {
		t.Fatalf("rows = %d, want 2", got)
	}
	var ticketRow *domain.DraftRow
	for i := range draft.Rows {
		if draft.Rows[i].Target.Kind == domain.TargetTicket {
			ticketRow = &draft.Rows[i]
		}
	}
	if ticketRow == nil {
		t.Fatal("ticket row missing")
	}
	if got := len(ticketRow.Cells); got != 2 {
		t.Errorf("ticket row cells = %d, want 2 (Mon+Tue)", got)
	}
	seenIDs := map[int]bool{}
	for _, c := range ticketRow.Cells {
		if c.Hours != 8.0 {
			t.Errorf("hours = %v, want 8.0", c.Hours)
		}
		seenIDs[c.SourceEntryID] = true
	}
	if !seenIDs[100] || !seenIDs[101] {
		t.Errorf("source IDs not preserved: %v", seenIDs)
	}
}

func TestBuildDraftFromReport_FiltersZeroPlaceholders(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	report := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
		Status:  domain.ReportOpen,
		Entries: []domain.TimeEntry{
			// Real entry — should produce a row + cell.
			{ID: 100, Date: week.AddDate(0, 0, 1), Minutes: 480,
				Target:   domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 1},
				TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true},
			// TD placeholder — should be silently dropped.
			{ID: 0, Date: time.Time{}, Minutes: 0,
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 999},
				TimeType: domain.TimeType{ID: 99, Name: "Other"}, Billable: false},
		},
	}
	draft := buildDraftFromReport("work", "default", report)
	if got := len(draft.Rows); got != 1 {
		t.Errorf("rows = %d, want 1 (zero placeholder should be filtered)", got)
	}
}

func TestComputeRemoteFingerprint_Stable(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	a := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week},
		Entries: []domain.TimeEntry{
			{ID: 1, Date: week.AddDate(0, 0, 1), Minutes: 60, ModifiedAt: time.Time{}},
			{ID: 2, Date: week.AddDate(0, 0, 2), Minutes: 30, ModifiedAt: time.Time{}},
		},
	}
	b := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week},
		Entries: []domain.TimeEntry{
			{ID: 2, Date: week.AddDate(0, 0, 2), Minutes: 30, ModifiedAt: time.Now()},
			{ID: 1, Date: week.AddDate(0, 0, 1), Minutes: 60, ModifiedAt: time.Now()},
		},
	}
	if computeRemoteFingerprint(a) != computeRemoteFingerprint(b) {
		t.Errorf("fingerprint not stable across order/modifiedAt")
	}
}
