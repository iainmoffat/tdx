package draftsvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

func TestService_SetArchived(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}
	if err := s.store.Save(d); err != nil {
		t.Fatal(err)
	}

	if err := s.SetArchived("work", week, "default", true); err != nil {
		t.Fatal(err)
	}
	loaded, _ := s.store.Load("work", week, "default")
	if !loaded.Archived {
		t.Errorf("Archived = false after SetArchived(true)")
	}

	if err := s.SetArchived("work", week, "default", false); err != nil {
		t.Fatal(err)
	}
	loaded, _ = s.store.Load("work", week, "default")
	if loaded.Archived {
		t.Errorf("Archived = true after SetArchived(false)")
	}
}
