package timesvc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/tdx"
)

// GetWeekReportForUser fetches the week-of-`date` report for a specific
// user (uid). Mirrors GetWeekReport's decoding pipeline. TD 401/403
// responses are mapped to errors that wrap domain.ErrPermission so the
// CLI can distinguish "you can't see this user's report" from genuine
// failures.
func (s *Service) GetWeekReportForUser(ctx context.Context, profileName string, date time.Time, uid string) (domain.WeekReport, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.WeekReport{}, err
	}

	day := date.In(domain.EasternTZ).Format("2006-01-02")
	path := fmt.Sprintf("/TDWebApi/api/time/report/%s/%s", day, uid)

	var wire wireTimeReport
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		if isPermissionErr(err) {
			return domain.WeekReport{}, fmt.Errorf("get week report for %s: %w", uid, domain.ErrPermission)
		}
		return domain.WeekReport{}, fmt.Errorf("get week report for %s: %w", uid, err)
	}

	periodStart := wire.PeriodStartDate.UTC()
	periodEnd := wire.PeriodEndDate.UTC()
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
		WeekRef:            ref,
		UserUID:            wire.TimeReportUid,
		TotalMinutes:       wire.MinutesTotal,
		MinutesBillable:    wire.MinutesBillable,
		MinutesNonBillable: wire.MinutesNonBillable,
		Status:             decodeReportStatus(wire.Status),
		Days:               buildDaySummaries(ref, entries),
		Entries:            entries,
	}, nil
}

// isPermissionErr matches the TD client's error types for 401/403.
// 401 → the client wraps tdx.ErrUnauthorized via fmt.Errorf("%w", ...).
// 403 → the client returns *tdx.APIError with Status=403; its Error()
// string contains "403". We check both sentinels so a future formatter
// change in the client doesn't silently break the 403 case.
func isPermissionErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, domain.ErrPermission) {
		return true
	}
	if errors.Is(err, tdx.ErrUnauthorized) {
		return true
	}
	var apiErr *tdx.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == 403
	}
	// Fallback: check the string for "403" in case the client wraps APIError.
	return strings.Contains(err.Error(), "403")
}
