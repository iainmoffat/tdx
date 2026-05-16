package draftsvc

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
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

func TestDraftStore_Load_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	weekStart := time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ)
	_, err := store.Load("default", weekStart, "../../credentials")
	if err == nil {
		t.Errorf("expected error for invalid name")
	}
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestDraftStore_Delete_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	weekStart := time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ)
	err := store.Delete("default", weekStart, "../../credentials")
	if err == nil {
		t.Errorf("expected error for invalid name")
	}
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestDraftStore_Exists_FalseForInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	weekStart := time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ)
	if store.Exists("default", weekStart, "../../credentials") {
		t.Errorf("expected Exists to return false for invalid name")
	}
}

func TestDraftStore_SaveNew_RejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(config.Paths{Root: dir})
	d := domain.WeekDraft{
		Profile:   "default",
		Name:      "../../foo",
		WeekStart: time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ),
	}
	err := store.SaveNew(d)
	if err == nil {
		t.Errorf("expected error for invalid name")
	}
	require.ErrorIs(t, err, domain.ErrInvalidArtifactName)
}

func TestDraftStore_List_SkipsInvalidNamesGracefully(t *testing.T) {
	dir := t.TempDir()
	paths := config.Paths{Root: dir}
	store := NewStore(paths)

	weekStart := time.Date(2026, 4, 12, 0, 0, 0, 0, domain.EasternTZ)
	require.NoError(t, store.Save(domain.WeekDraft{
		Profile: "default", Name: "valid", WeekStart: weekStart,
	}))

	// Drop a stray file with an invalid name into the same week dir.
	dateDir := weekStart.In(domain.EasternTZ).Format("2006-01-02")
	bogus := filepath.Join(paths.ProfileWeeksDir("default"), dateDir, "..bogus.yaml")
	require.NoError(t, os.WriteFile(bogus, []byte("profile: default\nname: ..bogus\nweekStart: 2026-04-12T00:00:00-04:00\n"), 0o600))

	drafts, err := store.List("default")
	require.NoError(t, err)
	require.Len(t, drafts, 1)
	require.Equal(t, "valid", drafts[0].Name)
}
