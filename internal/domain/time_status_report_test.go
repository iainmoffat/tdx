package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWeekStatusRow_HourMethods(t *testing.T) {
	r := WeekStatusRow{
		BillableMin:    300,
		NonBillableMin: 60,
		TotalMin:       360,
	}
	require.InDelta(t, 5.0, r.BillableHours(), 0.001)
	require.InDelta(t, 1.0, r.NonBillableHours(), 0.001)
	require.InDelta(t, 6.0, r.TotalHours(), 0.001)
}

func TestTimeStatusReport_TotalsAcrossRows(t *testing.T) {
	week1 := WeekRef{
		StartDate: time.Date(2026, 4, 12, 0, 0, 0, 0, EasternTZ),
		EndDate:   time.Date(2026, 4, 18, 0, 0, 0, 0, EasternTZ),
	}
	rep := TimeStatusReport{
		From: week1,
		To:   week1,
		Rows: []WeekStatusRow{
			{WeekRef: week1, BillableMin: 240, NonBillableMin: 0, TotalMin: 240},
			{WeekRef: week1, BillableMin: 60, NonBillableMin: 60, TotalMin: 120},
		},
	}
	bill, nonBill, tot := rep.Totals()
	require.Equal(t, 300, bill)
	require.Equal(t, 60, nonBill)
	require.Equal(t, 360, tot)
}

func TestTimeStatusReport_RowsByWeek_GroupsAndOrders(t *testing.T) {
	w1 := WeekRef{StartDate: time.Date(2026, 4, 12, 0, 0, 0, 0, EasternTZ)}
	w2 := WeekRef{StartDate: time.Date(2026, 4, 19, 0, 0, 0, 0, EasternTZ)}
	rep := TimeStatusReport{
		Rows: []WeekStatusRow{
			{WeekRef: w2, User: User{FullName: "Bob"}},
			{WeekRef: w1, User: User{FullName: "Alice"}},
			{WeekRef: w1, User: User{FullName: "Charlie"}},
		},
	}
	groups := rep.RowsByWeek()
	require.Len(t, groups, 2)
	require.True(t, groups[0].Week.StartDate.Before(groups[1].Week.StartDate),
		"groups sorted by week ascending")
	require.Equal(t, w1, groups[0].Week)
	// Within week, order preserves input (caller responsibility).
	require.Len(t, groups[0].Rows, 2)
}
