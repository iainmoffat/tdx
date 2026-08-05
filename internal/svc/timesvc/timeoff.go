package timesvc

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

const (
	// timeOffLookbackDays bounds how far back discovery searches for a prior
	// time-off entry to copy the tenant's time-off ItemID from.
	timeOffLookbackDays = 180
	// timeOffSearchLimit bounds how many entries the discovery search requests.
	timeOffSearchLimit = 500
)

// ResolveTimeOffItemID returns the time-off ItemID to use for a new time-off
// entry.
//
// TD models time off as a pseudo-project whose ID is tenant-specific (52 on the
// UF tenant) and is not exposed on any time-type listing, so tdx discovers it
// from the user's own recent time-off entries. An override > 0 short-circuits
// the lookup entirely and performs no API call.
//
// Returns an error wrapping domain.ErrTimeOffIDUnknown when the user has no
// time-off entry in the lookback window; the caller should then tell the user
// to log one in the TD web UI or pass an explicit ID.
func (s *Service) ResolveTimeOffItemID(ctx context.Context, profileName, userUID string, override int) (int, error) {
	if override > 0 {
		return override, nil
	}

	now := time.Now().In(domain.EasternTZ)
	filter := domain.EntryFilter{
		DateRange: domain.DateRange{
			From: now.AddDate(0, 0, -timeOffLookbackDays),
			To:   now,
		},
		UserUID: userUID,
		Limit:   timeOffSearchLimit,
	}

	entries, err := s.SearchEntries(ctx, profileName, filter)
	if err != nil {
		return 0, fmt.Errorf("discover time-off id: %w", err)
	}

	candidates := make([]domain.TimeEntry, 0, len(entries))
	for _, e := range entries {
		if e.Target.Kind == domain.TargetTimeOff && e.Target.ItemID > 0 {
			candidates = append(candidates, e)
		}
	}
	if len(candidates) == 0 {
		return 0, fmt.Errorf("%w: no time-off entry in the last %d days",
			domain.ErrTimeOffIDUnknown, timeOffLookbackDays)
	}

	// Most recent wins; ties break on the higher entry ID so the result is
	// deterministic when several entries share a date.
	sort.SliceStable(candidates, func(i, j int) bool {
		if !candidates[i].Date.Equal(candidates[j].Date) {
			return candidates[i].Date.After(candidates[j].Date)
		}
		return candidates[i].ID > candidates[j].ID
	})
	return candidates[0].Target.ItemID, nil
}
