package template

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ipm/tdx/internal/config"
	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/svc/tmplsvc"
	"github.com/stretchr/testify/require"
)

func TestDeleteCmd_Success(t *testing.T) {
	dir := seedTemplateDir(t)

	writeTestTemplate(t, dir, domain.Template{
		SchemaVersion: 1,
		Name:          "doomed",
		Rows: []domain.TemplateRow{
			{ID: "row1", Label: "Work", Hours: domain.WeekHours{Mon: 8}},
		},
	})

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"delete", "doomed"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), `deleted template "doomed"`)

	// Verify it's actually gone.
	paths := config.Paths{TemplatesDir: filepath.Join(dir, "templates")}
	store := tmplsvc.NewStore(paths)
	require.False(t, store.Exists("doomed"))
}

func TestDeleteCmd_NotFound(t *testing.T) {
	seedTemplateDir(t)

	cmd := NewCmd()
	cmd.SetArgs([]string{"delete", "nonexistent"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
