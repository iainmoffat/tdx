package template

import (
	"bytes"
	"testing"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/tmplsvc"
	"github.com/stretchr/testify/require"
)

func TestDeleteCmd_Success(t *testing.T) {
	seedTemplateDir(t)

	writeTestTemplate(t, "", domain.Template{
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
	paths, err := config.ResolvePaths()
	require.NoError(t, err)
	store := tmplsvc.NewStore(paths)
	require.False(t, store.Exists("default", "doomed"))
}

func TestDeleteCmd_NotFound(t *testing.T) {
	seedTemplateDir(t)

	cmd := NewCmd()
	cmd.SetArgs([]string{"delete", "nonexistent"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}
