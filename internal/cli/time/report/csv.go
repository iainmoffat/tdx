package report

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/iainmoffat/tdx/internal/domain"
)

// writeCSV emits a flat CSV (one row per WeekStatusRow). No subtotal
// rows — Excel pivots/SUMIF do the per-week math better than fixed rows.
func writeCSV(w io.Writer, rep domain.TimeStatusReport) error {
	cw := csv.NewWriter(w)
	header := []string{
		"weekStart", "weekEnd", "userUID", "name", "email",
		"reportsToName", "reportsToEmail", "status",
		"billableHours", "nonBillableHours", "totalHours",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range rep.Rows {
		if err := cw.Write([]string{
			r.WeekRef.StartDate.Format("2006-01-02"),
			r.WeekRef.EndDate.Format("2006-01-02"),
			r.User.UID,
			r.User.FullName,
			r.User.Email,
			r.User.ReportsToName,
			r.User.ReportsToEmail,
			string(r.Status),
			fmt.Sprintf("%.2f", r.BillableHours()),
			fmt.Sprintf("%.2f", r.NonBillableHours()),
			fmt.Sprintf("%.2f", r.TotalHours()),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
