package template

import (
	"bytes"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestListCmd_ShowsTemplates(t *testing.T) {
	dir := seedTemplateDir(t)

	writeTestTemplate(t, dir, domain.Template{
		SchemaVersion: 1,
		Name:          "standard-week",
		Description:   "My standard week",
		Rows: []domain.TemplateRow{
			{
				ID:    "row1",
				Label: "Project Alpha",
				Hours: domain.WeekHours{Mon: 8, Tue: 8, Wed: 8, Thu: 8, Fri: 8},
			},
		},
	})
	writeTestTemplate(t, dir, domain.Template{
		SchemaVersion: 1,
		Name:          "light-week",
		Description:   "Half day Fridays",
		Rows: []domain.TemplateRow{
			{
				ID:    "row1",
				Label: "Project Beta",
				Hours: domain.WeekHours{Mon: 8, Tue: 8, Wed: 8, Thu: 8, Fri: 4},
			},
			{
				ID:    "row2",
				Label: "Admin",
				Hours: domain.WeekHours{Fri: 4},
			},
		},
	})

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, "standard-week")
	require.Contains(t, got, "light-week")
	require.Contains(t, got, "NAME")
	require.Contains(t, got, "ROWS")
	require.Contains(t, got, "HOURS")
}

func TestListCmd_EmptyDir(t *testing.T) {
	seedTemplateDir(t)

	var out bytes.Buffer
	cmd := NewCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())

	require.Contains(t, out.String(), "no templates saved")
}

func TestListCmd_JSON(t *testing.T) {
	dir := seedTemplateDir(t)

	writeTestTemplate(t, dir, domain.Template{
		SchemaVersion: 1,
		Name:          "json-test",
		Description:   "For JSON output",
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
	cmd.SetArgs([]string{"list", "--json"})
	require.NoError(t, cmd.Execute())

	got := out.String()
	require.Contains(t, got, `"schema": "tdx.v1.templateList"`)
	require.Contains(t, got, `"json-test"`)
}
