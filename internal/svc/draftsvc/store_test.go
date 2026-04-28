package draftsvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

func TestStore_SaveLoad(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := NewStore(paths)

	draft := domain.WeekDraft{
		SchemaVersion: 1, Profile: "work", Name: "default",
		WeekStart: time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ), // Sunday
		Rows:      []domain.DraftRow{{ID: "row-01"}},
	}
	if err := s.Save(draft); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load("work", draft.WeekStart, "default")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Name != "default" {
		t.Errorf("name lost")
	}

	if !s.Exists("work", draft.WeekStart, "default") {
		t.Errorf("Exists = false after Save")
	}
}

func TestStore_List(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := NewStore(paths)

	week1 := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	week2 := time.Date(2026, 5, 10, 0, 0, 0, 0, domain.EasternTZ)
	for _, d := range []domain.WeekDraft{
		{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week1},
		{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week2},
	} {
		if err := s.Save(d); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	drafts, err := s.List("work")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(drafts) != 2 {
		t.Errorf("List returned %d drafts, want 2", len(drafts))
	}
}

func TestStore_Delete(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := NewStore(paths)
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}
	if err := s.Save(d); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("work", week, "default"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s.Exists("work", week, "default") {
		t.Errorf("Exists = true after Delete")
	}
}

func TestStore_LoadMissing(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := NewStore(paths)
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	if _, err := s.Load("work", week, "default"); err == nil {
		t.Errorf("expected error loading non-existent draft")
	}
}

func TestStore_SaveNew_RefusesCollision(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := NewStore(paths)
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}
	if err := s.SaveNew(d); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveNew(d); err == nil {
		t.Errorf("SaveNew should refuse to overwrite existing draft")
	}
}
