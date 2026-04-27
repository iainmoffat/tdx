package draftsvc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
)

func TestMigrate_SingleProfile_AutoYes(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, "templates")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "canonical.yaml"), []byte("name: canonical\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	paths := config.Paths{Root: home, LegacyTemplatesDir: legacy}
	profiles := []string{"work"}

	result, err := Migrate(paths, profiles, "work", &silentPrompter{})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !result.Migrated {
		t.Errorf("Migrated = false, want true")
	}

	target := paths.ProfileTemplatesDir("work")
	if _, err := os.Stat(filepath.Join(target, "canonical.yaml")); err != nil {
		t.Errorf("expected canonical.yaml in %s: %v", target, err)
	}

	if _, err := os.Stat(filepath.Join(legacy, ".migrated")); err != nil {
		t.Errorf(".migrated marker missing: %v", err)
	}

	result2, err := Migrate(paths, profiles, "work", &silentPrompter{})
	if err != nil {
		t.Fatalf("re-Migrate: %v", err)
	}
	if result2.Migrated {
		t.Errorf("second Migrate Migrated = true, want false (already done)")
	}
}

type silentPrompter struct{}

func (silentPrompter) Confirm(question string) (bool, error) { return true, nil }

func TestMigrate_MultiProfile_PromptsAndRespects(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, "templates")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "x.yaml"), []byte("name: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	paths := config.Paths{Root: home, LegacyTemplatesDir: legacy}
	profiles := []string{"work", "personal"}

	decline := &fakePrompter{answer: false}
	result, err := Migrate(paths, profiles, "work", decline)
	if err != nil {
		t.Fatal(err)
	}
	if result.Migrated {
		t.Errorf("Migrated = true, want false (declined)")
	}

	if _, err := os.Stat(filepath.Join(legacy, ".migrated")); err == nil {
		t.Errorf(".migrated written despite decline")
	}
	if _, err := os.Stat(filepath.Join(legacy, "x.yaml")); err != nil {
		t.Errorf("legacy file removed despite decline: %v", err)
	}
}

type fakePrompter struct{ answer bool }

func (f fakePrompter) Confirm(string) (bool, error) { return f.answer, nil }
