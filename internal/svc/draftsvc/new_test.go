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
