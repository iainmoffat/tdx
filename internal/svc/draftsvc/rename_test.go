package draftsvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

func TestService_Rename_Success(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "old", WeekStart: week}
	if err := s.store.Save(d); err != nil { t.Fatal(err) }
	if err := s.store.SavePulledSnapshot(d); err != nil { t.Fatal(err) }
	if _, err := s.snapshots.Take(d, OpManual, ""); err != nil { t.Fatal(err) }

	if err := s.Rename("work", week, "old", "new"); err != nil { t.Fatal(err) }

	if !s.store.Exists("work", week, "new") { t.Errorf("new draft missing after rename") }
	if s.store.Exists("work", week, "old") { t.Errorf("old draft still present after rename") }

	if _, err := s.store.LoadPulledSnapshot("work", week, "new"); err != nil {
		t.Errorf("pulled snapshot missing for new: %v", err)
	}
	list, err := s.snapshots.List("work", week, "new")
	if err != nil { t.Fatal(err) }
	if len(list) < 2 {
		t.Errorf("snapshots list = %d, want >= 2 (manual + pre-rename)", len(list))
	}
	var hasPreRename bool
	for _, sn := range list {
		if sn.Op == OpPreRename { hasPreRename = true }
	}
	if !hasPreRename {
		t.Errorf("no pre-rename snapshot found")
	}

	loaded, err := s.store.Load("work", week, "new")
	if err != nil { t.Fatal(err) }
	if loaded.Name != "new" {
		t.Errorf("YAML name = %q, want new", loaded.Name)
	}
}

func TestService_Rename_RefusesCollision(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	a := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "a", WeekStart: week}
	b := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "b", WeekStart: week}
	if err := s.store.Save(a); err != nil { t.Fatal(err) }
	if err := s.store.Save(b); err != nil { t.Fatal(err) }

	if err := s.Rename("work", week, "a", "b"); err == nil {
		t.Errorf("Rename should refuse when target exists")
	}
	if !s.store.Exists("work", week, "a") { t.Errorf("source disappeared on failed rename") }
	if !s.store.Exists("work", week, "b") { t.Errorf("destination disappeared on failed rename") }
}
