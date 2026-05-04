package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
)

// printText renders a human-readable Time Status Report.
func printText(w io.Writer, rep domain.TimeStatusReport, f statusFlags) error {
	groups := rep.RowsByWeek()
	for _, g := range groups {
		_, _ = fmt.Fprintf(w, "WEEK %s — %s\n",
			g.Week.StartDate.Format("2006-01-02"),
			g.Week.EndDate.Format("2006-01-02"))

		headers := []string{"NAME", "EMAIL", "REPORTS TO", "STATUS", "BILL", "NON-BILL", "TOTAL"}
		rows := make([][]string, 0, len(g.Rows))
		var billSum, nonBillSum, totalSum int
		for _, r := range g.Rows {
			rows = append(rows, []string{
				r.User.FullName,
				r.User.Email,
				r.User.ReportsToName,
				string(r.Status),
				fmt.Sprintf("%.2f", r.BillableHours()),
				fmt.Sprintf("%.2f", r.NonBillableHours()),
				fmt.Sprintf("%.2f", r.TotalHours()),
			})
			billSum += r.BillableMin
			nonBillSum += r.NonBillableMin
			totalSum += r.TotalMin
		}
		summary := []string{"TOTAL", "", "", "",
			fmt.Sprintf("%.2f", float64(billSum)/60.0),
			fmt.Sprintf("%.2f", float64(nonBillSum)/60.0),
			fmt.Sprintf("%.2f", float64(totalSum)/60.0),
		}
		render.Table(w, headers, rows, summary)
		_, _ = fmt.Fprintln(w)
	}
	bill, nonBill, total := rep.Totals()
	_, _ = fmt.Fprintf(w, "OVERALL: %.2f bill · %.2f non-bill · %.2f total\n",
		float64(bill)/60.0, float64(nonBill)/60.0, float64(total)/60.0)
	return nil
}

// printJSON emits the tdx.v1.timeStatusReport envelope.
func printJSON(w io.Writer, rep domain.TimeStatusReport, f statusFlags) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(buildJSONEnvelope(rep, f))
}

// buildJSONEnvelope returns the same value printJSON encodes. Exposed
// internally so MCP can re-use the envelope without re-encoding.
func buildJSONEnvelope(rep domain.TimeStatusReport, f statusFlags) any {
	type filterJSON struct {
		Selector     string   `json:"selector"`
		Users        []string `json:"users,omitempty"`
		Manager      string   `json:"manager,omitempty"`
		Account      string   `json:"account,omitempty"`
		ResourcePool string   `json:"resourcePool,omitempty"`
		Incomplete   bool     `json:"incomplete,omitempty"`
		Threshold    float64  `json:"threshold,omitempty"`
		From         string   `json:"from"`
		To           string   `json:"to"`
	}
	type rowJSON struct {
		UserUID          string  `json:"userUID"`
		Name             string  `json:"name"`
		Email            string  `json:"email"`
		ReportsToName    string  `json:"reportsToName,omitempty"`
		ReportsToEmail   string  `json:"reportsToEmail,omitempty"`
		Status           string  `json:"status"`
		BillableHours    float64 `json:"billableHours"`
		NonBillableHours float64 `json:"nonBillableHours"`
		TotalHours       float64 `json:"totalHours"`
	}
	type totalsJSON struct {
		BillableHours    float64 `json:"billableHours"`
		NonBillableHours float64 `json:"nonBillableHours"`
		TotalHours       float64 `json:"totalHours"`
	}
	type weekJSON struct {
		WeekStart string     `json:"weekStart"`
		WeekEnd   string     `json:"weekEnd"`
		Rows      []rowJSON  `json:"rows"`
		Subtotals totalsJSON `json:"subtotals"`
	}

	selector := selectorOf(f)
	filter := filterJSON{
		Selector: selector,
		From:     rep.From.StartDate.Format("2006-01-02"),
		To:       rep.To.EndDate.Format("2006-01-02"),
	}
	switch selector {
	case "user":
		filter.Users = f.users
	case "manager":
		filter.Manager = f.manager
	case "account":
		filter.Account = f.account
	case "resource-pool":
		filter.ResourcePool = f.resourcePool
	}
	if f.incomplete {
		filter.Incomplete = true
		filter.Threshold = f.threshold
		if filter.Threshold <= 0 {
			filter.Threshold = 40
		}
	}

	weeks := []weekJSON{}
	for _, g := range rep.RowsByWeek() {
		var bill, nonBill, total int
		rows := make([]rowJSON, 0, len(g.Rows))
		for _, r := range g.Rows {
			rows = append(rows, rowJSON{
				UserUID: r.User.UID, Name: r.User.FullName, Email: r.User.Email,
				ReportsToName: r.User.ReportsToName, ReportsToEmail: r.User.ReportsToEmail,
				Status:        string(r.Status),
				BillableHours: r.BillableHours(), NonBillableHours: r.NonBillableHours(), TotalHours: r.TotalHours(),
			})
			bill += r.BillableMin
			nonBill += r.NonBillableMin
			total += r.TotalMin
		}
		weeks = append(weeks, weekJSON{
			WeekStart: g.Week.StartDate.Format("2006-01-02"),
			WeekEnd:   g.Week.EndDate.Format("2006-01-02"),
			Rows:      rows,
			Subtotals: totalsJSON{
				BillableHours: float64(bill) / 60.0, NonBillableHours: float64(nonBill) / 60.0, TotalHours: float64(total) / 60.0,
			},
		})
	}

	bill, nonBill, total := rep.Totals()
	envelope := struct {
		Schema string     `json:"schema"`
		Filter filterJSON `json:"filter"`
		Weeks  []weekJSON `json:"weeks"`
		Totals totalsJSON `json:"totals"`
	}{
		Schema: "tdx.v1.timeStatusReport",
		Filter: filter,
		Weeks:  weeks,
		Totals: totalsJSON{
			BillableHours: float64(bill) / 60.0, NonBillableHours: float64(nonBill) / 60.0, TotalHours: float64(total) / 60.0,
		},
	}
	return envelope
}

func selectorOf(f statusFlags) string {
	switch {
	case len(f.users) > 0:
		return "user"
	case f.manager != "":
		return "manager"
	case f.account != "":
		return "account"
	case f.resourcePool != "":
		return "resource-pool"
	case f.all:
		return "all"
	}
	return ""
}
