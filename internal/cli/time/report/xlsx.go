package report

import (
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/xuri/excelize/v2"
)

// writeXLSX writes a TimeStatusReport to path as an .xlsx file.
// Single sheet "Time Status Report" with bold frozen header row.
// Hours are stored as numbers (not strings) so Excel can pivot/sum.
func writeXLSX(path string, rep domain.TimeStatusReport) error {
	f := excelize.NewFile()
	defer f.Close()

	const sheet = "Time Status Report"
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return fmt.Errorf("new sheet: %w", err)
	}
	f.SetActiveSheet(idx)
	if err := f.DeleteSheet("Sheet1"); err != nil {
		// Non-fatal: leave the default sheet if delete fails.
		_ = err
	}

	headers := []string{
		"weekStart", "weekEnd", "userUID", "name", "email",
		"reportsToName", "reportsToEmail", "status",
		"billableHours", "nonBillableHours", "totalHours",
	}

	// Bold style for header.
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		return fmt.Errorf("header style: %w", err)
	}

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cell, cell, headerStyle); err != nil {
			return err
		}
	}

	for ri, r := range rep.Rows {
		row := ri + 2 // 1-indexed; header takes row 1
		stringVals := []any{
			r.WeekRef.StartDate.Format("2006-01-02"),
			r.WeekRef.EndDate.Format("2006-01-02"),
			r.User.UID,
			r.User.FullName,
			r.User.Email,
			r.User.ReportsToName,
			r.User.ReportsToEmail,
			string(r.Status),
		}
		for i, v := range stringVals {
			cell, _ := excelize.CoordinatesToCellName(i+1, row)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
		// Numeric columns
		numericVals := []float64{r.BillableHours(), r.NonBillableHours(), r.TotalHours()}
		for i, v := range numericVals {
			cell, _ := excelize.CoordinatesToCellName(len(stringVals)+1+i, row)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
	}

	// Sensible static column widths.
	widths := map[string]float64{
		"A": 12, "B": 12, "C": 16, "D": 24, "E": 28, "F": 24, "G": 28,
		"H": 14, "I": 12, "J": 14, "K": 12,
	}
	for col, w := range widths {
		_ = f.SetColWidth(sheet, col, col, w)
	}

	// Freeze the top row.
	_ = f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	return f.SaveAs(path)
}
