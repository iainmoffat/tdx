package tmplsvc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func testPaths(t *testing.T) config.Paths {
	t.Helper()
	dir := t.TempDir()
	return config.Paths{
		Root:            dir,
		ConfigFile:      filepath.Join(dir, "config.yaml"),
		CredentialsFile: filepath.Join(dir, "credentials.yaml"),
		TemplatesDir:    filepath.Join(dir, "templates"),
	}
}

func sampleTemplate() domain.Template {
	return domain.Template{
		Name:        "default-week",
		Description: "Typical work week",
		Rows: []domain.TemplateRow{
			{
				ID:         "row-01",
				TimeTypeID: 5,
				Target:     domain.Target{Kind: domain.TargetProject, ItemID: 54},
				Hours:      domain.WeekHours{Mon: 8.0, Tue: 8.0, Wed: 8.0, Thu: 8.0, Fri: 8.0},
			},
		},
	}
}

func TestStore_SaveAndLoad(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	tmpl := sampleTemplate()
	require.NoError(t, store.Save(tmpl))

	loaded, err := store.Load("default-week")
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

	require.NoError(t, store.Save(tmpl1))
	require.NoError(t, store.Save(tmpl2))

	templates, err := store.List()
	require.NoError(t, err)
	require.Len(t, templates, 2)
}

func TestStore_Delete(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	require.NoError(t, store.Save(sampleTemplate()))
	require.True(t, store.Exists("default-week"))

	require.NoError(t, store.Delete("default-week"))
	require.False(t, store.Exists("default-week"))
}

func TestStore_Load_NotFound(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	_, err := store.Load("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestStore_List_EmptyDir(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	templates, err := store.List()
	require.NoError(t, err)
	require.Empty(t, templates)
}

func TestStore_CanonicalYAML(t *testing.T) {
	paths := testPaths(t)
	store := NewStore(paths)

	tmpl := sampleTemplate()
	require.NoError(t, store.Save(tmpl))

	data, err := os.ReadFile(filepath.Join(paths.TemplatesDir, "default-week.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(data), "name: default-week")
	require.Contains(t, string(data), "description: Typical work week")
}
