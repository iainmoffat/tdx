package draftsvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

func TestService_NewBlank(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	d, err := s.NewBlank("work", week, "default")
	if err != nil {
		t.Fatal(err)
	}
	if d.Profile != "work" || d.Name != "default" || !d.WeekStart.Equal(week) {
		t.Errorf("identity wrong: %+v", d)
	}
	if d.Provenance.Kind != domain.ProvenanceBlank {
		t.Errorf("Provenance.Kind = %s, want blank", d.Provenance.Kind)
	}
	if len(d.Rows) != 0 {
		t.Errorf("blank draft has %d rows, want 0", len(d.Rows))
	}

	if _, err := s.NewBlank("work", week, "default"); err == nil {
		t.Errorf("NewBlank should refuse on collision")
	}
}

func TestService_NewFromTemplate(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	tmpl := domain.Template{
		SchemaVersion: 1, Name: "canonical",
		Rows: []domain.TemplateRow{
			{ID: "row-01",
				Target:   domain.Target{Kind: domain.TargetTicket, AppID: 42, ItemID: 1},
				TimeType: domain.TimeType{ID: 7, Name: "Work"}, Billable: true,
				Hours: domain.WeekHours{Mon: 8, Tue: 8}},
		},
	}

	d, err := s.NewFromTemplate("work", week, "default", tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if d.Provenance.Kind != domain.ProvenanceFromTemplate {
		t.Errorf("Provenance.Kind = %s, want from-template", d.Provenance.Kind)
	}
	if d.Provenance.FromTemplate != "canonical" {
		t.Errorf("Provenance.FromTemplate = %q, want canonical", d.Provenance.FromTemplate)
	}
	if len(d.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(d.Rows))
	}
	if len(d.Rows[0].Cells) != 2 {
		t.Errorf("cells = %d, want 2 (Mon+Tue)", len(d.Rows[0].Cells))
	}
}

func TestService_NewFromDraft(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	srcWeek := time.Date(2026, 4, 26, 0, 0, 0, 0, domain.EasternTZ)
	dstWeek := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	src := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: srcWeek,
		Provenance: domain.DraftProvenance{Kind: domain.ProvenanceBlank},
		Rows: []domain.DraftRow{{
			ID:    "row-01",
			Cells: []domain.DraftCell{{Day: time.Monday, Hours: 4, SourceEntryID: 999}},
		}},
	}
	if err := s.store.Save(src); err != nil {
		t.Fatal(err)
	}

	d, err := s.NewFromDraft("work", dstWeek, "default", "work", srcWeek, "default")
	if err != nil {
		t.Fatal(err)
	}
	if !d.WeekStart.Equal(dstWeek) {
		t.Errorf("dst weekStart = %v, want %v", d.WeekStart, dstWeek)
	}
	if d.Provenance.Kind != domain.ProvenanceFromDraft {
		t.Errorf("Provenance.Kind = %s, want from-draft", d.Provenance.Kind)
	}
	if d.Provenance.ShiftedByDays != 7 {
		t.Errorf("ShiftedByDays = %d, want 7", d.Provenance.ShiftedByDays)
	}
	if len(d.Rows) != 1 || len(d.Rows[0].Cells) != 1 {
		t.Errorf("rows/cells lost in clone")
	}
	if d.Rows[0].Cells[0].SourceEntryID != 0 {
		t.Errorf("clone should drop sourceEntryIDs (was %d)", d.Rows[0].Cells[0].SourceEntryID)
	}
}
