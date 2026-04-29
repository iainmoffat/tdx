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

func TestBuildDraftFromReport_KeepsPlaceholderRowsAsEmpty(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	report := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
		Status:  domain.ReportOpen,
		Entries: []domain.TimeEntry{
			// Real entry — should produce a row + cell.
			{ID: 100, Date: week.AddDate(0, 0, 1), Minutes: 480,
				Target:   domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 1},
				TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true},
			// TD placeholder — should produce an empty row (editable but no cells).
			{ID: 0, Date: time.Time{}, Minutes: 0,
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 999},
				TimeType: domain.TimeType{ID: 99, Name: "Other"}, Billable: false},
		},
	}
	draft := buildDraftFromReport("work", "default", report)
	if got := len(draft.Rows); got != 2 {
		t.Fatalf("rows = %d, want 2 (real entry + placeholder both produce rows)", got)
	}

	var placeholderRow *domain.DraftRow
	for i := range draft.Rows {
		if draft.Rows[i].Target.Kind == domain.TargetProject {
			placeholderRow = &draft.Rows[i]
		}
	}
	if placeholderRow == nil {
		t.Fatal("placeholder row missing — should be kept for editing")
	}
	if got := len(placeholderRow.Cells); got != 0 {
		t.Errorf("placeholder row cells = %d, want 0 (empty row, editable)", got)
	}
	if placeholderRow.TimeType.ID != 99 {
		t.Errorf("placeholder row TimeType.ID = %d, want 99", placeholderRow.TimeType.ID)
	}
}

func TestBuildDraftFromReport_PlaceholderAndRealEntryCollapseToOneRow(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	target := domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123}
	tt := domain.TimeType{ID: 7, Name: "Work"}

	report := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
		Status:  domain.ReportOpen,
		Entries: []domain.TimeEntry{
			// Placeholder for the same (target, type, billable) — comes first.
			{ID: 0, Minutes: 0, Target: target, TimeType: tt, Billable: true},
			// Real entry on Monday.
			{ID: 100, Date: week.AddDate(0, 0, 1), Minutes: 480,
				Target: target, TimeType: tt, Billable: true},
		},
	}
	draft := buildDraftFromReport("work", "default", report)

	if got := len(draft.Rows); got != 1 {
		t.Fatalf("rows = %d, want 1 (placeholder collapses with real entry on same key)", got)
	}
	if got := len(draft.Rows[0].Cells); got != 1 {
		t.Errorf("cells = %d, want 1 (only the real entry contributes a cell)", got)
	}
	if draft.Rows[0].Cells[0].SourceEntryID != 100 {
		t.Errorf("cell SourceEntryID = %d, want 100", draft.Rows[0].Cells[0].SourceEntryID)
	}
}

func TestBuildDraftFromReport_KeepsPlaceholderWithZeroTypeID(t *testing.T) {
	// Type-less placeholders are kept here; Service.Pull's
	// resolveDefaultTimeTypes assigns a TimeType post-hoc.
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	report := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
		Status:  domain.ReportOpen,
		Entries: []domain.TimeEntry{
			{ID: 0, Minutes: 0,
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 999},
				TimeType: domain.TimeType{ID: 0}, Billable: false},
		},
	}
	draft := buildDraftFromReport("work", "default", report)
	if got := len(draft.Rows); got != 1 {
		t.Fatalf("rows = %d, want 1 (type-less placeholder retained for resolution)", got)
	}
	if got := draft.Rows[0].TimeType.ID; got != 0 {
		t.Errorf("TimeType.ID = %d, want 0 (resolution happens at the service layer)", got)
	}
}

func TestBuildDraftFromReport_KeepsPlaceholderWithValidTypeID(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	report := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
		Status:  domain.ReportOpen,
		Entries: []domain.TimeEntry{
			// Placeholder with a valid TimeType — should produce an empty editable row.
			{ID: 0, Minutes: 0,
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 555},
				TimeType: domain.TimeType{ID: 5, Name: "Standard Activities"}, Billable: false},
		},
	}
	draft := buildDraftFromReport("work", "default", report)
	if got := len(draft.Rows); got != 1 {
		t.Fatalf("rows = %d, want 1 (placeholder with valid TimeType.ID kept)", got)
	}
	if got := len(draft.Rows[0].Cells); got != 0 {
		t.Errorf("cells = %d, want 0 (placeholder produces empty row)", got)
	}
	if got := draft.Rows[0].TimeType.ID; got != 5 {
		t.Errorf("TimeType.ID = %d, want 5", got)
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
