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
}

func TestWeekDraft_Validate(t *testing.T) {
	valid := WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default",
		WeekStart: time.Date(2026, 5, 3, 0, 0, 0, 0, EasternTZ),
		Rows: []DraftRow{{ID: "row-01", Target: Target{Kind: TargetProject, ItemID: 1}}},
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
			Rows: []DraftRow{{ID: "row-01"}, {ID: "row-01"}}}},
	}
	for _, c := range cases {
		if err := c.d.Validate(); err == nil {
			t.Errorf("%s: expected error", c.name)
		}
	}
}
