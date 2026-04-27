package domain

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestWeekDraft_YAMLRoundTrip(t *testing.T) {
	in := WeekDraft{
		SchemaVersion: 1,
		Profile:       "work",
		WeekStart:     time.Date(2026, 5, 3, 0, 0, 0, 0, EasternTZ),
		Name:          "default",
		Notes:         "Friday short week.",
		Provenance: DraftProvenance{
			Kind:              ProvenancePulled,
			PulledAt:          time.Date(2026, 4, 27, 13, 12, 14, 0, time.UTC),
			RemoteFingerprint: "8a7fc2e1",
			RemoteStatus:      ReportOpen,
		},
		Rows: []DraftRow{{
			ID:       "row-01",
			Target:   Target{Kind: TargetTicket, AppID: 42, ItemID: 123, DisplayName: "Big Project"},
			TimeType: TimeType{ID: 7, Name: "Work"},
			Billable: true,
			Cells: []DraftCell{
				{Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
				{Day: time.Tuesday, Hours: 8.0, SourceEntryID: 98732},
			},
		}},
	}

	data, err := yaml.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out WeekDraft
	if err := yaml.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out.Name != in.Name {
		t.Errorf("Name lost in round-trip")
	}
	if !out.WeekStart.Equal(in.WeekStart) {
		t.Errorf("WeekStart lost: %v vs %v", out.WeekStart, in.WeekStart)
	}
	if len(out.Rows) != 1 || len(out.Rows[0].Cells) != 2 {
		t.Errorf("Rows/Cells lost: %+v", out.Rows)
	}
	if out.Rows[0].Cells[0].SourceEntryID != 98731 {
		t.Errorf("SourceEntryID lost")
	}

	if out.SchemaVersion != in.SchemaVersion {
		t.Errorf("SchemaVersion lost: got %d, want %d", out.SchemaVersion, in.SchemaVersion)
	}
	if out.Profile != in.Profile {
		t.Errorf("Profile lost: got %q, want %q", out.Profile, in.Profile)
	}
	if out.Notes != in.Notes {
		t.Errorf("Notes lost: got %q, want %q", out.Notes, in.Notes)
	}
	if out.Provenance.Kind != in.Provenance.Kind {
		t.Errorf("Provenance.Kind lost: got %q, want %q", out.Provenance.Kind, in.Provenance.Kind)
	}
	if !out.Provenance.PulledAt.Equal(in.Provenance.PulledAt) {
		t.Errorf("Provenance.PulledAt lost: got %v, want %v", out.Provenance.PulledAt, in.Provenance.PulledAt)
	}
	if out.Provenance.RemoteFingerprint != in.Provenance.RemoteFingerprint {
		t.Errorf("Provenance.RemoteFingerprint lost")
	}
	if out.Provenance.RemoteStatus != in.Provenance.RemoteStatus {
		t.Errorf("Provenance.RemoteStatus lost")
	}
	if out.Rows[0].Target.Kind != in.Rows[0].Target.Kind {
		t.Errorf("Target.Kind lost")
	}
	if out.Rows[0].Target.AppID != in.Rows[0].Target.AppID {
		t.Errorf("Target.AppID lost")
	}
	if out.Rows[0].TimeType.ID != in.Rows[0].TimeType.ID {
		t.Errorf("TimeType.ID lost")
	}
	if out.Rows[0].Billable != in.Rows[0].Billable {
		t.Errorf("Billable lost")
	}
}

func TestWeekDraft_Validate(t *testing.T) {
	valid := WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default",
		WeekStart: time.Date(2026, 5, 3, 0, 0, 0, 0, EasternTZ),
		Rows:      []DraftRow{{ID: "row-01", Target: Target{Kind: TargetProject, ItemID: 1}}},
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid draft errored: %v", err)
	}

	cases := []struct {
		name string
		d    WeekDraft
	}{
		{"missing profile", WeekDraft{SchemaVersion: 1, Name: "x", WeekStart: valid.WeekStart}},
		{"missing name", WeekDraft{SchemaVersion: 1, Profile: "work", WeekStart: valid.WeekStart}},
		{"zero weekStart", WeekDraft{SchemaVersion: 1, Profile: "work", Name: "x"}},
		{"weekStart not Sunday", WeekDraft{SchemaVersion: 1, Profile: "work", Name: "x",
			WeekStart: time.Date(2026, 5, 4, 0, 0, 0, 0, EasternTZ)}},
		{"duplicate row IDs", WeekDraft{SchemaVersion: 1, Profile: "work", Name: "x",
			WeekStart: valid.WeekStart,
			Rows:      []DraftRow{{ID: "row-01"}, {ID: "row-01"}}}},
	}
	for _, c := range cases {
		if err := c.d.Validate(); err == nil {
			t.Errorf("%s: expected error", c.name)
		}
	}
}

func TestComputeCellState(t *testing.T) {
	pulled := DraftCell{Day: time.Monday, Hours: 8.0, SourceEntryID: 98731}

	cases := []struct {
		name    string
		pulled  DraftCell
		current DraftCell
		want    CellState
	}{
		{"untouched", pulled, pulled, CellUntouched},
		{"edited (hours)", pulled, DraftCell{Day: time.Monday, Hours: 6.0, SourceEntryID: 98731}, CellEdited},
		{"edited (cleared = delete-on-push)", pulled,
			DraftCell{Day: time.Monday, Hours: 0, SourceEntryID: 98731}, CellEdited},
		{"added (no source)", DraftCell{}, DraftCell{Day: time.Monday, Hours: 4.0}, CellAdded},
	}
	for _, c := range cases {
		got := ComputeCellState(c.pulled, c.current)
		if got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestComputeSyncState(t *testing.T) {
	weekStart := time.Date(2026, 5, 4, 0, 0, 0, 0, EasternTZ)
	pulledFingerprint := "abc123"

	base := WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: weekStart,
		Provenance: DraftProvenance{Kind: ProvenancePulled, RemoteFingerprint: pulledFingerprint},
	}

	pulledCells := map[string]DraftCell{
		"row-01:Monday": {Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
	}

	// Clean: cells match what was pulled, fingerprint matches.
	cleanDraft := base
	cleanDraft.Rows = []DraftRow{{ID: "row-01", Cells: []DraftCell{
		{Day: time.Monday, Hours: 8.0, SourceEntryID: 98731},
	}}}

	if state := ComputeSyncState(cleanDraft, pulledCells, pulledFingerprint); state.Sync != SyncClean {
		t.Errorf("clean: got %s, want clean", state.Sync)
	}

	// Dirty: cell hours edited.
	dirty := cleanDraft
	dirty.Rows[0].Cells[0].Hours = 6.0
	if state := ComputeSyncState(dirty, pulledCells, pulledFingerprint); state.Sync != SyncDirty {
		t.Errorf("dirty: got %s, want dirty", state.Sync)
	}
	if state := ComputeSyncState(dirty, pulledCells, pulledFingerprint); state.Stale {
		t.Errorf("dirty: got Stale=true, want false (fingerprint matches)")
	}

	// Stale: fingerprint differs.
	if state := ComputeSyncState(cleanDraft, pulledCells, "different"); !state.Stale {
		t.Errorf("stale: got Stale=false, want true")
	}
}
