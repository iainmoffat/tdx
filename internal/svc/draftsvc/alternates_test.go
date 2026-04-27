package draftsvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

func TestStore_AlternateNamesIsolated(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := NewStore(paths)
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	a := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week, Notes: "primary"}
	b := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "pristine", WeekStart: week, Notes: "alt"}
	if err := s.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(b); err != nil {
		t.Fatal(err)
	}

	if !s.Exists("work", week, "default") {
		t.Errorf("default missing")
	}
	if !s.Exists("work", week, "pristine") {
		t.Errorf("pristine missing")
	}

	loadedA, err := s.Load("work", week, "default")
	if err != nil {
		t.Fatal(err)
	}
	loadedB, err := s.Load("work", week, "pristine")
	if err != nil {
		t.Fatal(err)
	}
	if loadedA.Notes == loadedB.Notes {
		t.Errorf("alternates not isolated: both have notes %q", loadedA.Notes)
	}

	list, err := s.List("work")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("List returned %d, want 2", len(list))
	}
}
