package template

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/domain"
)

func TestEditCmd_NotFound(t *testing.T) {
	_ = seedTemplateDir(t)
	cmd := newEditCmd()
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestTemplateToSheet_RoundTrip(t *testing.T) {
	tmpl := domain.Template{
		Name: "demo",
		Rows: []domain.TemplateRow{
			{ID: "row-01", Label: "B", Target: domain.Target{GroupName: "Group A"}, TimeType: domain.TimeType{Name: "Type"}, Hours: domain.WeekHours{Mon: 4}},
			{ID: "row-02", Label: "A", Target: domain.Target{GroupName: "Group A"}, TimeType: domain.TimeType{Name: "Type"}, Hours: domain.WeekHours{Tue: 2}},
			{ID: "row-03", Label: "C", Target: domain.Target{GroupName: "Group B"}, TimeType: domain.TimeType{Name: "Type"}, Hours: domain.WeekHours{Wed: 1}},
		},
	}

	sheet := templateToSheet(tmpl)
	require.Equal(t, "demo", sheet.Name)
	// Sorted by GroupName then Label: Group A:A, Group A:B, Group B:C
	require.Equal(t, []string{"row-02", "row-01", "row-03"},
		[]string{sheet.Rows[0].ID, sheet.Rows[1].ID, sheet.Rows[2].ID})

	// Edit a sheet row's hours, apply back.
	sheet.Rows[0].Hours.Tue = 99
	applySheetToTemplate(sheet, &tmpl)

	// row-02 (Label "A") should now have Tue=99.
	for _, r := range tmpl.Rows {
		if r.ID == "row-02" {
			require.InDelta(t, 99.0, r.Hours.Tue, 0.001)
		}
	}
}
