package timesvc

import (
	"context"
	"fmt"
	"time"

	"github.com/ipm/tdx/internal/domain"
)

// GetWeekReport fetches TD's weekly report for the week containing the
// given date. The per-day breakdown (report.Days) is computed client-side
// from the flat Times array TD returns, since TD does not include a
// daily summary in the response.
func (s *Service) GetWeekReport(ctx context.Context, profileName string, date time.Time) (domain.WeekReport, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.WeekReport{}, err
	}

	// TD's URL path wants YYYY-MM-DD. Any day in the target week works;
	// TD normalizes to the period containing that date.
	day := date.In(domain.EasternTZ).Format("2006-01-02")
	path := fmt.Sprintf("/TDWebApi/api/time/report/%s", day)

	var wire wireTimeReport
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return domain.WeekReport{}, fmt.Errorf("get week report: %w", err)
	}

	// TD returns PeriodStartDate/PeriodEndDate as midnight UTC. Converting
	// directly with .In(EasternTZ) would yield 8pm the previous calendar day
	// in EDT. Normalize to midnight EasternTZ by extracting the date fields.
	periodStart := wire.PeriodStartDate.Time.UTC()
	periodEnd := wire.PeriodEndDate.Time.UTC()
	ref := domain.WeekRef{
		StartDate: time.Date(periodStart.Year(), periodStart.Month(), periodStart.Day(), 0, 0, 0, 0, domain.EasternTZ),
		EndDate:   time.Date(periodEnd.Year(), periodEnd.Month(), periodEnd.Day(), 0, 0, 0, 0, domain.EasternTZ),
	}

	entries := make([]domain.TimeEntry, 0, len(wire.Times))
	for _, t := range wire.Times {
		entry, err := decodeTimeEntry(t)
		if err != nil {
			return domain.WeekReport{}, err
		}
		entries = append(entries, entry)
	}
	if err := s.resolveTimeTypeNames(ctx, profileName, entries); err != nil {
		return domain.WeekReport{}, err
	}

	return domain.WeekReport{
		WeekRef:      ref,
		UserUID:      wire.TimeReportUid,
		TotalMinutes: wire.MinutesTotal,
		Status:       decodeReportStatus(wire.Status),
		Days:         buildDaySummaries(ref, entries),
		Entries:      entries,
	}, nil
}

// timeDateToEasternMidnight converts a TD TimeDate value (returned as midnight
// UTC) to midnight Eastern time on the same calendar date. TD always encodes
// TimeDate as "YYYY-MM-DDT00:00:00Z"; the UTC date is the canonical date for
// the entry regardless of what time zone the client is in.
func timeDateToEasternMidnight(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, domain.EasternTZ)
}

// GetLockedDays returns the locked days in [from, to] inclusive. TD's
// response is a flat array of ISO8601 date strings (midnight UTC); we
// normalize each to midnight EasternTZ via timeDateToEasternMidnight so
// the calendar date is preserved across the UTC→Eastern boundary.
func (s *Service) GetLockedDays(ctx context.Context, profileName string, from, to time.Time) ([]domain.LockedDay, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}

	fromStr := from.In(domain.EasternTZ).Format("2006-01-02")
	toStr := to.In(domain.EasternTZ).Format("2006-01-02")
	path := fmt.Sprintf("/TDWebApi/api/time/locked?startDate=%s&endDate=%s", fromStr, toStr)

	var wire []tdTime
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("get locked days: %w", err)
	}

	out := make([]domain.LockedDay, 0, len(wire))
	for _, ts := range wire {
		out = append(out, domain.LockedDay{Date: timeDateToEasternMidnight(ts.Time)})
	}
	return out, nil
}

// buildDaySummaries produces seven consecutive DaySummary entries covering
// Sun..Sat of the week in ref, with minutes accumulated from entries that
// fall within each day. Days with zero entries still appear with Minutes=0.
func buildDaySummaries(ref domain.WeekRef, entries []domain.TimeEntry) []domain.DaySummary {
	days := make([]domain.DaySummary, 7)
	for i := 0; i < 7; i++ {
		days[i] = domain.DaySummary{
			Date: ref.StartDate.AddDate(0, 0, i),
		}
	}
	for _, e := range entries {
		// Both e.Date and ref.StartDate are midnight EasternTZ. We can't use
		// Sub().Hours()/24 because spring-forward weeks have a 23-hour gap and
		// fall-back weeks have a 25-hour gap. Compare calendar dates instead.
		ey, em, ed := e.Date.Date()
		ry, rm, rd := ref.StartDate.Date()
		entryDay := time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC)
		refDay := time.Date(ry, rm, rd, 0, 0, 0, 0, time.UTC)
		dayIdx := int(entryDay.Sub(refDay).Hours() / 24)
		if dayIdx < 0 || dayIdx >= 7 {
			continue // entry falls outside the reported week; should not happen
		}
		days[dayIdx].Minutes += e.Minutes
	}
	return days
}
