package tmplsvc

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func testPaths(t *testing.T) config.Paths {
	t.Helper()
	dir := t.TempDir()
	return config.Paths{
		Root:               dir,
		ConfigFile:         filepath.Join(dir, "config.yaml"),
		CredentialsFile:    filepath.Join(dir, "credentials.yaml"),
		LegacyTemplatesDir: filepath.Join(dir, "templates"),
	}
}

func sampleTemplate() domain.Template {
	return domain.Template{
		SchemaVersion: 1,
		Name:          "default-week",
		Description:   "Typical work week",
		Rows: []domain.TemplateRow{
			{
				ID:       "row-01",
				Label:    "Project work",
				Target:   domain.Target{Kind: domain.TargetProject, ItemID: 54},
				TimeType: domain.TimeType{ID: 5, Name: "Development"},
				Hours:    domain.WeekHours{Mon: 8.0, Tue: 8.0, Wed: 8.0, Thu: 8.0, Fri: 8.0},
			},
		},
	}
}

// TestStore_PerProfile_Isolation verifies the core Phase A fix:
//   - A template saved under "work" is found by Load("work", name).
//   - Load("other", name) does NOT find it (no cross-profile leakage).
//   - A template written directly into the legacy dir is found via fallback
//     from Load("work", legacyName).
//   - List("work") merges both per-profile and legacy templates without duplicates.
func TestStore_PerProfile_Isolation(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	// Save a template under profile "work".
	tmpl := domain.Template{
		SchemaVersion: 1,
		Name:          "per-profile-tmpl",
		Description:   "Per-profile template",
		Rows:          []domain.TemplateRow{{ID: "r1", Hours: domain.WeekHours{Mon: 8}}},
	}
	require.NoError(t, store.Save("work", tmpl))

	// Load("work", name) must succeed.
	loaded, err := store.Load("work", "per-profile-tmpl")
	require.NoError(t, err)
	require.Equal(t, "per-profile-tmpl", loaded.Name)

	// Load("other", name) must NOT find it.
	_, err = store.Load("other", "per-profile-tmpl")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")

	// Write a template directly into the legacy dir (simulates pre-migration state).
	legacyDir := paths.LegacyTemplatesDir
	require.NoError(t, os.MkdirAll(legacyDir, 0o700))
	legacyTmpl := domain.Template{
		SchemaVersion: 1,
		Name:          "legacy-tmpl",
		Description:   "Legacy template",
		Rows:          []domain.TemplateRow{{ID: "r2", Hours: domain.WeekHours{Tue: 4}}},
	}
	legacyData, err := yaml.Marshal(legacyTmpl)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "legacy-tmpl.yaml"), legacyData, 0o600))

	// Load("work", legacyName) must find it via fallback.
	legacyLoaded, err := store.Load("work", "legacy-tmpl")
	require.NoError(t, err)
	require.Equal(t, "legacy-tmpl", legacyLoaded.Name)

	// List("work") must return both per-profile and legacy templates.
	all, err := store.List("work")
	require.NoError(t, err)
	require.Len(t, all, 2, "expected per-profile-tmpl + legacy-tmpl")

	names := make(map[string]bool)
	for _, t := range all {
		names[t.Name] = true
	}
	require.True(t, names["per-profile-tmpl"])
	require.True(t, names["legacy-tmpl"])

	// Duplicate shadowing: save "legacy-tmpl" under "work" profile; List should still return 2.
	require.NoError(t, store.Save("work", legacyTmpl))
	all2, err := store.List("work")
	require.NoError(t, err)
	require.Len(t, all2, 2, "per-profile copy should shadow legacy; still 2 unique templates")
}

func TestStore_SaveAndLoad(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	tmpl := sampleTemplate()
	require.NoError(t, store.Save("default", tmpl))

	loaded, err := store.Load("default", "default-week")
	require.NoError(t, err)
	require.Equal(t, "default-week", loaded.Name)
	require.Len(t, loaded.Rows, 1)
	require.Equal(t, "row-01", loaded.Rows[0].ID)
	require.InDelta(t, 8.0, loaded.Rows[0].Hours.Mon, 0.001)
	require.Equal(t, domain.TargetProject, loaded.Rows[0].Target.Kind)
	require.Equal(t, 54, loaded.Rows[0].Target.ItemID)
}

func TestStore_List(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	tmpl1 := sampleTemplate()
	tmpl2 := sampleTemplate()
	tmpl2.Name = "light-week"
	tmpl2.Description = "Part time"

	require.NoError(t, store.Save("default", tmpl1))
	require.NoError(t, store.Save("default", tmpl2))

	templates, err := store.List("default")
	require.NoError(t, err)
	require.Len(t, templates, 2)
}

func TestStore_Delete(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	require.NoError(t, store.Save("default", sampleTemplate()))
	require.True(t, store.Exists("default", "default-week"))

	require.NoError(t, store.Delete("default", "default-week"))
	require.False(t, store.Exists("default", "default-week"))
}

func TestStore_Load_NotFound(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	_, err := store.Load("default", "nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestStore_List_EmptyDir(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	templates, err := store.List("default")
	require.NoError(t, err)
	require.Empty(t, templates)
}

func TestStore_CanonicalYAML(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	tmpl := sampleTemplate()
	require.NoError(t, store.Save("default", tmpl))

	profileDir := paths.ProfileTemplatesDir("default")
	data, err := os.ReadFile(filepath.Join(profileDir, "default-week.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(data), "name: default-week")
	require.Contains(t, string(data), "description: Typical work week")
}
