package template

import (
	"bytes"
	"testing"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestShowCmd_RendersGrid(t *testing.T) {
	dir := seedTemplateDir(t)

	writeTestTemplate(t, dir, domain.Template{
		SchemaVersion: 1,
		Name:          "my-week",
		Description:   "Standard work week",
		Rows: []domain.TemplateRow{
			{
				ID:    "row1",
				Label: "Project Alpha",
				Target: domain.Target{
					Kind: domain.TargetTicket,
				},
				TimeType: domain.TimeType{
					Name: "Development",
				},
				Hours: domain.WeekHours{Mon: 8, Tue: 8, Wed: 8, Thu: 8, Fri: 8},
				ResolverHints: domain.ResolverHints{
					TargetDisplayName: "Ingest pipeline",
				},
			},
		},
	})

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "my-week"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "my-week")
	require.Contains(t, got, "Standard work week")
	require.Contains(t, got, "ROW")
	require.Contains(t, got, "Project Alpha")
	require.Contains(t, got, "Development")
	require.Contains(t, got, "Ingest pipeline")
}

func TestShowCmd_NotFound(t *testing.T) {
	seedTemplateDir(t)

	cmd := NewCmd()
	cmd.SetArgs([]string{"show", "nonexistent"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestShowCmd_JSON(t *testing.T) {
	dir := seedTemplateDir(t)

	writeTestTemplate(t, dir, domain.Template{
		SchemaVersion: 1,
		Name:          "json-show",
		Rows: []domain.TemplateRow{
			{
				ID:    "row1",
				Label: "Work",
				Hours: domain.WeekHours{Mon: 8},
			},
		},
	})

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"show", "json-show", "--json"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, `"schema": "tdx.v1.template"`)
	require.Contains(t, got, `"json-show"`)
}
