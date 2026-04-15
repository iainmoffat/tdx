package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
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

func TestWeekGrid_SpringForwardBucketing(t *testing.T) {
	// Spring-forward DST starts 2026-03-08 02:00 EST → 03:00 EDT.
	// The week is 2026-03-08 (Sun) to 2026-03-14 (Sat).
	ref := domain.WeekRef{
		StartDate: time.Date(2026, 3, 8, 0, 0, 0, 0, domain.EasternTZ),
		EndDate:   time.Date(2026, 3, 14, 0, 0, 0, 0, domain.EasternTZ),
	}
	report := domain.WeekReport{
		WeekRef:      ref,
		TotalMinutes: 60,
		Status:       domain.ReportOpen,
		Days: []domain.DaySummary{
			{Date: ref.StartDate.AddDate(0, 0, 0)},
			{Date: ref.StartDate.AddDate(0, 0, 1), Minutes: 60},
			{Date: ref.StartDate.AddDate(0, 0, 2)},
			{Date: ref.StartDate.AddDate(0, 0, 3)},
			{Date: ref.StartDate.AddDate(0, 0, 4)},
			{Date: ref.StartDate.AddDate(0, 0, 5)},
			{Date: ref.StartDate.AddDate(0, 0, 6)},
		},
		Entries: []domain.TimeEntry{
			{
				Target:   domain.Target{DisplayRef: "#9999", DisplayName: "DST test"},
				TimeType: domain.TimeType{Name: "Development"},
				// Monday after the spring-forward gap.
				Date:    ref.StartDate.AddDate(0, 0, 1),
				Minutes: 60,
			},
		},
	}

	var buf bytes.Buffer
	WeekGrid(&buf, report)
	got := buf.String()

	// The entry must land in MON's column (1.0), not SUN.
	// Find the data row for #9999 and inspect it.
	var dataLine string
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "#9999") {
			dataLine = line
			break
		}
	}
	require.NotEmpty(t, dataLine, "expected to find a data row for #9999")

	// Find SUN's column position in the header row, then verify the data row
	// has "." (not "1.0") at that position.
	headerLine := ""
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "SUN") && strings.Contains(line, "MON") {
			headerLine = line
			break
		}
	}
	require.NotEmpty(t, headerLine)
	sunIdx := strings.Index(headerLine, "SUN")
	require.GreaterOrEqual(t, sunIdx, 0)

	// The data row must contain "1.0" (the Monday entry rendered as hours).
	require.Contains(t, dataLine, "1.0", "expected 1.0 hours rendered for the entry")

	// At the SUN column position in the data row, we should see "." (empty),
	// not "1.0". A simple byte check at the same column offset suffices.
	require.True(t, len(dataLine) > sunIdx, "data line shorter than expected")
	require.Equal(t, byte('.'), dataLine[sunIdx], "Sunday cell should be empty (.); got %q at idx %d in %q", dataLine[sunIdx], sunIdx, dataLine)
}

func TestGrid_BasicTemplate(t *testing.T) {
	data := GridData{
		Title: "default-week — Typical work week",
		Rows: []GridRow{
			{
				Label:  "Platform standup",
				Detail: "General Admin · Platform Services",
				Ref:    "(project)",
				Hours:  [7]float64{0, 1.0, 1.0, 1.0, 1.0, 1.0, 0},
			},
			{
				Label:  "Ingest pipeline",
				Detail: "Development · IT Help Desk",
				Ref:    "(ticket #12345)",
				Hours:  [7]float64{0, 3.0, 3.0, 3.0, 3.0, 3.0, 0},
			},
		},
	}

	var buf bytes.Buffer
	Grid(&buf, data)
	got := buf.String()

	require.Contains(t, got, "default-week")
	require.Contains(t, got, "Platform standup")
	require.Contains(t, got, "Ingest pipeline")
	require.Contains(t, got, "General Admin")
	require.Contains(t, got, "SUN")
	require.Contains(t, got, "SAT")
	require.Contains(t, got, "DAY TOTAL")
	require.Contains(t, got, "20.0") // 5+15 total
}

func TestGrid_WithMarkers(t *testing.T) {
	data := GridData{
		Title: "Apply preview",
		Rows: []GridRow{
			{
				Label:   "Work item",
				Detail:  "Dev",
				Hours:   [7]float64{0, 2.0, 2.0, 0, 0, 0, 0},
				Markers: [7]string{"", "+", "=", "", "", "", ""},
			},
		},
	}

	var buf bytes.Buffer
	Grid(&buf, data)
	got := buf.String()

	require.Contains(t, got, "+2.0")
	require.Contains(t, got, "=2.0")
}

func TestGrid_EmptyRows(t *testing.T) {
	data := GridData{
		Title: "Empty",
		Rows:  nil,
	}

	var buf bytes.Buffer
	Grid(&buf, data)
	got := buf.String()

	require.Contains(t, got, "Empty")
	require.Contains(t, got, "no rows")
}

func TestGrid_WithSubtitle(t *testing.T) {
	data := GridData{
		Title:    "test-template — My template",
		Subtitle: "(derived from 2026-03-15)",
		Rows: []GridRow{
			{
				Label: "Row 1",
				Hours: [7]float64{0, 8.0, 8.0, 8.0, 8.0, 8.0, 0},
			},
		},
	}

	var buf bytes.Buffer
	Grid(&buf, data)
	got := buf.String()

	require.Contains(t, got, "test-template")
	require.Contains(t, got, "(derived from 2026-03-15)")
	require.Contains(t, got, "40.0") // 5*8 total
}
