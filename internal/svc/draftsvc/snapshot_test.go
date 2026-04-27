package draftsvc

import (
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

func TestSnapshotStore_TakeAndList(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	ss := NewSnapshotStore(paths, 10)
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}

	s1, err := ss.Take(d, OpPrePull, "")
	if err != nil {
		t.Fatalf("Take: %v", err)
	}
	s2, err := ss.Take(d, OpPrePush, "")
	if err != nil {
		t.Fatalf("Take: %v", err)
	}
	if s1.Sequence == s2.Sequence {
		t.Errorf("sequences not incrementing")
	}
	list, err := ss.List("work", week, "default")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List returned %d, want 2", len(list))
	}
}

func TestSnapshotStore_RetentionPrunes(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	ss := NewSnapshotStore(paths, 3)
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}

	for i := 0; i < 5; i++ {
		if _, err := ss.Take(d, OpManual, ""); err != nil {
			t.Fatal(err)
		}
	}
	list, err := ss.List("work", week, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("after 5 takes with retention=3 got %d", len(list))
	}
}

func TestSnapshotStore_PinnedSurvivesPrune(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	ss := NewSnapshotStore(paths, 2)
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)
	d := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week}

	s1, _ := ss.Take(d, OpManual, "")
	if err := ss.Pin("work", week, "default", s1.Sequence, "important"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		ss.Take(d, OpManual, "") //nolint:errcheck
	}

	list, _ := ss.List("work", week, "default")
	found := false
	for _, s := range list {
		if s.Sequence == s1.Sequence && s.Pinned {
			found = true
		}
	}
	if !found {
		t.Errorf("pinned snapshot pruned")
	}
}

func TestService_RestoreSnapshot(t *testing.T) {
	paths := config.Paths{Root: t.TempDir()}
	s := newServiceWithTimeWriter(paths, &mockTimeWriter{})
	week := time.Date(2026, 5, 3, 0, 0, 0, 0, domain.EasternTZ)

	base := domain.WeekDraft{SchemaVersion: 1, Profile: "work", Name: "default", WeekStart: week, Notes: "v1"}
	if err := s.store.Save(base); err != nil {
		t.Fatal(err)
	}
	snap, err := s.snapshots.Take(base, OpManual, "")
	if err != nil {
		t.Fatal(err)
	}

	// Mutate.
	base.Notes = "v2"
	if err := s.store.Save(base); err != nil {
		t.Fatal(err)
	}

	// Restore.
	if err := s.RestoreSnapshot("work", week, "default", snap.Sequence); err != nil {
		t.Fatal(err)
	}

	restored, _ := s.store.Load("work", week, "default")
	if restored.Notes != "v1" {
		t.Errorf("Notes = %q after restore, want v1", restored.Notes)
	}

	list, _ := s.snapshots.List("work", week, "default")
	var hasPreRestore bool
	for _, sn := range list {
		if sn.Op == OpPreRestore {
			hasPreRestore = true
		}
	}
	if !hasPreRestore {
		t.Errorf("no pre-restore snapshot taken")
	}
}
