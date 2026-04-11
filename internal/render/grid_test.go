package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestWeekGrid_RendersSevenColumns(t *testing.T) {
	ref := domain.WeekRef{
		StartDate: time.Date(2026, 4, 5, 0, 0, 0, 0, domain.EasternTZ),
		EndDate:   time.Date(2026, 4, 11, 0, 0, 0, 0, domain.EasternTZ),
	}
	report := domain.WeekReport{
		WeekRef:      ref,
		TotalMinutes: 1200, // 20 hours
		Status:       domain.ReportOpen,
		Days: []domain.DaySummary{
			{Date: ref.StartDate.AddDate(0, 0, 0), Minutes: 0},   // Sun
			{Date: ref.StartDate.AddDate(0, 0, 1), Minutes: 240}, // Mon 4h
			{Date: ref.StartDate.AddDate(0, 0, 2), Minutes: 240}, // Tue 4h
			{Date: ref.StartDate.AddDate(0, 0, 3), Minutes: 240}, // Wed 4h
			{Date: ref.StartDate.AddDate(0, 0, 4), Minutes: 240}, // Thu 4h
			{Date: ref.StartDate.AddDate(0, 0, 5), Minutes: 240}, // Fri 4h
			{Date: ref.StartDate.AddDate(0, 0, 6), Minutes: 0},   // Sat
		},
		Entries: []domain.TimeEntry{
			{
				Target:   domain.Target{DisplayRef: "#12345", DisplayName: "Ingest pipeline"},
				TimeType: domain.TimeType{Name: "Development"},
				Date:     ref.StartDate.AddDate(0, 0, 1),
				Minutes:  240,
			},
			{
				Target:   domain.Target{DisplayRef: "#12345", DisplayName: "Ingest pipeline"},
				TimeType: domain.TimeType{Name: "Development"},
				Date:     ref.StartDate.AddDate(0, 0, 2),
				Minutes:  240,
			},
			{
				Target:   domain.Target{DisplayRef: "#12345", DisplayName: "Ingest pipeline"},
				TimeType: domain.TimeType{Name: "Development"},
				Date:     ref.StartDate.AddDate(0, 0, 3),
				Minutes:  240,
			},
			{
				Target:   domain.Target{DisplayRef: "#12345", DisplayName: "Ingest pipeline"},
				TimeType: domain.TimeType{Name: "Development"},
				Date:     ref.StartDate.AddDate(0, 0, 4),
				Minutes:  240,
			},
			{
				Target:   domain.Target{DisplayRef: "#12345", DisplayName: "Ingest pipeline"},
				TimeType: domain.TimeType{Name: "Development"},
				Date:     ref.StartDate.AddDate(0, 0, 5),
				Minutes:  240,
			},
		},
	}

	var buf bytes.Buffer
	WeekGrid(&buf, report)
	got := buf.String()

	// Header row with all seven weekday abbreviations.
	require.Contains(t, got, "SUN")
	require.Contains(t, got, "MON")
	require.Contains(t, got, "TUE")
	require.Contains(t, got, "WED")
	require.Contains(t, got, "THU")
	require.Contains(t, got, "FRI")
	require.Contains(t, got, "SAT")
	require.Contains(t, got, "TOTAL")

	// Week header line containing the date range and status.
	require.Contains(t, got, "2026-04-05")
	require.Contains(t, got, "2026-04-11")
	require.Contains(t, got, "open")

	// Row for the ticket with its sub-label.
	require.Contains(t, got, "#12345 Ingest pipeline")
	require.Contains(t, got, "└ Development")

	// Day-total row.
	require.Contains(t, got, "DAY TOTAL")

	// Empty cells render as "." for visual scanning.
	lines := strings.Split(got, "\n")
	var sawDot bool
	for _, line := range lines {
		if strings.Contains(line, "#12345") && strings.Contains(line, ".") {
			sawDot = true
		}
	}
	require.True(t, sawDot, "expected empty Sunday/Saturday cells to render as '.'")
}

func TestWeekGrid_EmptyReport(t *testing.T) {
	ref := domain.WeekRef{
		StartDate: time.Date(2026, 4, 5, 0, 0, 0, 0, domain.EasternTZ),
		EndDate:   time.Date(2026, 4, 11, 0, 0, 0, 0, domain.EasternTZ),
	}
	report := domain.WeekReport{
		WeekRef: ref,
		Status:  domain.ReportOpen,
		Days: []domain.DaySummary{
			{Date: ref.StartDate.AddDate(0, 0, 0)},
			{Date: ref.StartDate.AddDate(0, 0, 1)},
			{Date: ref.StartDate.AddDate(0, 0, 2)},
			{Date: ref.StartDate.AddDate(0, 0, 3)},
			{Date: ref.StartDate.AddDate(0, 0, 4)},
			{Date: ref.StartDate.AddDate(0, 0, 5)},
			{Date: ref.StartDate.AddDate(0, 0, 6)},
		},
	}

	var buf bytes.Buffer
	WeekGrid(&buf, report)
	got := buf.String()

	require.Contains(t, got, "no entries in this week")
}
