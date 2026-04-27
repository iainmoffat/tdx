package draftsvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestReconcile_DeleteOnClearedPulledCell(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	pulledRow := domain.DraftRow{
		ID: "row-01",
		Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
		TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
			{Day: time.Tuesday, Hours: 0, SourceEntryID: 98732},
		},
	}
	draft := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
		Rows: []domain.DraftRow{pulledRow},
		Provenance: domain.DraftProvenance{
			Kind: domain.ProvenancePulled, RemoteFingerprint: "fp1",
		},
	}
	pulled := map[string]domain.DraftCell{
		"row-01:Monday":  {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
		"row-01:Tuesday": {Day: time.Tuesday, Hours: 8.0, SourceEntryID: 98732},
	}
	report := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
		Status:  domain.ReportOpen,
		Entries: []domain.TimeEntry{
			{ID: 98731, Date: week.AddDate(0, 0, 1), Minutes: 480,
				Target: pulledRow.Target, TimeType: pulledRow.TimeType, Billable: true},
			{ID: 98732, Date: week.AddDate(0, 0, 2), Minutes: 480,
				Target: pulledRow.Target, TimeType: pulledRow.TimeType, Billable: true},
		},
	}

	diff, err := reconcileDraft(draft, pulled, report, []domain.LockedDay{}, "fp1", "user-1")
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var hasDelete bool
	for _, a := range diff.Actions {
		if a.Kind == domain.ActionDelete && a.DeleteEntryID == 98732 {
			hasDelete = true
		}
	}
	if !hasDelete {
		t.Errorf("expected ActionDelete for entry 98732, got %+v", diff.Actions)
	}

	var hasMondaySkip bool
	for _, a := range diff.Actions {
		if a.Kind == domain.ActionSkip && a.Date.Weekday() == time.Monday {
			hasMondaySkip = true
		}
	}
	if !hasMondaySkip {
		t.Errorf("expected skip for untouched Monday")
	}
}

func TestReconcile_HashStableAndIncludesWatermark(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	row := domain.DraftRow{
		ID: "row-01",
		Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
		TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
		Cells: []domain.DraftCell{{Day: time.Monday, Hours: 6.0, SourceEntryID: 98731}},
	}
	draft := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
		Rows: []domain.DraftRow{row},
		Provenance: domain.DraftProvenance{Kind: domain.ProvenancePulled, RemoteFingerprint: "fpA"},
	}
	pulled := map[string]domain.DraftCell{
		"row-01:Monday": {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
	}
	report := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
		Status:  domain.ReportOpen,
		Entries: []domain.TimeEntry{
			{ID: 98731, Date: week.AddDate(0, 0, 1), Minutes: 480,
				Target: row.Target, TimeType: row.TimeType, Billable: true},
		},
	}

	diff1, err := reconcileDraft(draft, pulled, report, nil, "fpA", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	diff2, err := reconcileDraft(draft, pulled, report, nil, "fpA", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if diff1.DiffHash != diff2.DiffHash {
		t.Errorf("hash unstable across runs: %s vs %s", diff1.DiffHash, diff2.DiffHash)
	}
	diff3, err := reconcileDraft(draft, pulled, report, nil, "fpDIFFERENT", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if diff1.DiffHash == diff3.DiffHash {
		t.Errorf("hash did not change with watermark; expected different hashes")
	}
}

func TestReconcile_LockedDayBlocks(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	monday := week.AddDate(0, 0, 1)
	row := domain.DraftRow{
		ID: "row-01",
		Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
		TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
		Cells: []domain.DraftCell{{Day: time.Monday, Hours: 6.0, SourceEntryID: 98731}},
	}
	draft := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
		Rows: []domain.DraftRow{row},
		Provenance: domain.DraftProvenance{Kind: domain.ProvenancePulled, RemoteFingerprint: "fp"},
	}
	pulled := map[string]domain.DraftCell{
		"row-01:Monday": {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
	}
	report := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
		Status:  domain.ReportOpen,
		Entries: []domain.TimeEntry{
			{ID: 98731, Date: monday, Minutes: 480, Target: row.Target,
				TimeType: row.TimeType, Billable: true},
		},
	}
	locked := []domain.LockedDay{{Date: monday}}

	diff, err := reconcileDraft(draft, pulled, report, locked, "fp", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Actions) != 0 {
		t.Errorf("expected 0 actions for locked day, got %d", len(diff.Actions))
	}
	if len(diff.Blockers) != 1 || diff.Blockers[0].Kind != domain.BlockerLocked {
		t.Errorf("expected 1 BlockerLocked, got %+v", diff.Blockers)
	}
}

func TestReconcile_SubmittedWeekRefuses(t *testing.T) {
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	row := domain.DraftRow{
		ID: "row-01",
		Target: domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 123},
		TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
		Cells: []domain.DraftCell{
			{Day: time.Monday, Hours: 6.0, SourceEntryID: 98731},
			{Day: time.Tuesday, Hours: 4.0},
		},
	}
	draft := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week,
		Rows: []domain.DraftRow{row},
		Provenance: domain.DraftProvenance{Kind: domain.ProvenancePulled, RemoteFingerprint: "fp"},
	}
	pulled := map[string]domain.DraftCell{
		"row-01:Monday": {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
	}
	report := domain.WeekReport{
		WeekRef: domain.WeekRef{StartDate: week, EndDate: week.AddDate(0, 0, 6)},
		Status:  domain.ReportSubmitted,
		Entries: []domain.TimeEntry{
			{ID: 98731, Date: week.AddDate(0, 0, 1), Minutes: 480, Target: row.Target,
				TimeType: row.TimeType, Billable: true},
		},
	}

	diff, err := reconcileDraft(draft, pulled, report, nil, "fp", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Actions) != 0 {
		t.Errorf("submitted week: expected 0 actions, got %d", len(diff.Actions))
	}
	if len(diff.Blockers) != 2 {
		t.Errorf("submitted week: expected 2 blockers (one per non-untouched cell), got %d", len(diff.Blockers))
	}
	for _, b := range diff.Blockers {
		if b.Kind != domain.BlockerSubmitted {
			t.Errorf("expected BlockerSubmitted, got %s", b.Kind)
		}
	}
}
