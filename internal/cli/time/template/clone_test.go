package template

import (
	"bytes"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
	"github.com/stretchr/testify/require"
)

func TestCloneCmd_Success(t *testing.T) {
	seedTemplateDir(t)

	writeTestTemplate(t, "", domain.Template{
		SchemaVersion: 1,
		Name:          "original",
		Description:   "The original template",
		Rows: []domain.TemplateRow{
			{ID: "row1", Label: "Work", Hours: domain.WeekHours{Mon: 8, Tue: 8}},
		},
	})

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"clone", "original", "copy"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), "cloned")
	require.Contains(t, out.String(), "original")
	require.Contains(t, out.String(), "copy")

	// Verify the clone exists and has the correct name.
	paths, err := config.ResolvePaths()
	require.NoError(t, err)
	store := tmplsvc.NewStore(paths)
	require.True(t, store.Exists("default", "copy"))

	cloned, err := store.Load("default", "copy")
	require.NoError(t, err)
	require.Equal(t, "copy", cloned.Name)
	require.Equal(t, "The original template", cloned.Description)
	require.Nil(t, cloned.DerivedFrom)
	require.Len(t, cloned.Rows, 1)
}

func TestCloneCmd_DestExists(t *testing.T) {
	seedTemplateDir(t)

	writeTestTemplate(t, "", domain.Template{
		SchemaVersion: 1,
		Name:          "src",
		Rows: []domain.TemplateRow{
			{ID: "row1", Label: "Work", Hours: domain.WeekHours{Mon: 8}},
		},
	})
	writeTestTemplate(t, "", domain.Template{
		SchemaVersion: 1,
		Name:          "dst",
		Rows: []domain.TemplateRow{
			{ID: "row1", Label: "Other", Hours: domain.WeekHours{Mon: 4}},
		},
	})

	cmd := NewCmd()
	cmd.SetArgs([]string{"clone", "src", "dst"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestCloneCmd_SourceNotFound(t *testing.T) {
	seedTemplateDir(t)

	cmd := NewCmd()
	cmd.SetArgs([]string{"clone", "nonexistent", "copy"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
