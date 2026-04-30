package report

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"golang.org/x/sync/errgroup"
)

// timesvcAPI is the subset of timesvc.Service the runner needs.
type timesvcAPI interface {
	GetWeekReportForUser(ctx context.Context, profile string, date time.Time, uid string) (domain.WeekReport, error)
}

// peoplesvcAPI is the subset of peoplesvc.Service the runner needs.
type peoplesvcAPI interface {
	GetUser(ctx context.Context, profile, uid string) (domain.User, error)
	SearchUsers(ctx context.Context, profile string, filter domain.UserFilter) ([]domain.User, error)
}

// authsvcAPI is the subset of authsvc.Service the runner needs.
type authsvcAPI interface {
	WhoAmI(ctx context.Context, profile string) (domain.User, error)
}

// runnerDeps bundles the service dependencies for assembleReport so tests
// can inject mocks via the interfaces above.
type runnerDeps struct {
	Time    timesvcAPI
	People  peoplesvcAPI
	Auth    authsvcAPI
	Profile string
}

const (
	maxConcurrency = 5
	hardLimit      = 1000
)

// assembleReport orchestrates the per-(user, week) fan-out and returns
// the assembled TimeStatusReport. Permission errors become rows with
// Status="permission-denied" and zero hours; any other error fails the run.
func assembleReport(ctx context.Context, deps runnerDeps, f statusFlags) (domain.TimeStatusReport, error) {
	weeks, err := resolveWeeks(f)
	if err != nil {
		return domain.TimeStatusReport{}, err
	}

	users, err := resolveUsers(ctx, deps, f)
	if err != nil {
		return domain.TimeStatusReport{}, err
	}

	// Apply --limit cap.
	cap := f.limit
	if cap <= 0 || cap > hardLimit {
		cap = hardLimit
	}
	if len(users) > cap {
		users = users[:cap]
	}

	// Fan out per-(user, week).
	resultsMu := sync.Mutex{}
	results := make([]domain.WeekStatusRow, 0, len(users)*len(weeks))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	for _, week := range weeks {
		week := week
		for _, u := range users {
			u := u
			g.Go(func() error {
				rep, err := deps.Time.GetWeekReportForUser(gctx, deps.Profile, week.StartDate, u.UID)
				row := domain.WeekStatusRow{
					WeekRef: week,
					User:    u,
				}
				switch {
				case err == nil:
					row.Status = rep.Status
					row.BillableMin = rep.MinutesBillable
					row.NonBillableMin = rep.MinutesNonBillable
					row.TotalMin = rep.TotalMinutes
				case errors.Is(err, domain.ErrPermission):
					row.Status = domain.ReportStatus("permission-denied")
				default:
					return fmt.Errorf("get report for %s/%s: %w", u.UID, week.StartDate.Format("2006-01-02"), err)
				}
				resultsMu.Lock()
				results = append(results, row)
				resultsMu.Unlock()
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return domain.TimeStatusReport{}, err
	}

	// Apply --include-zero filter.
	if !f.includeZero {
		filtered := results[:0]
		for _, r := range results {
			if r.TotalMin > 0 {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Sort: by week ascending, then by user FullName.
	sort.SliceStable(results, func(i, j int) bool {
		if !results[i].WeekRef.StartDate.Equal(results[j].WeekRef.StartDate) {
			return results[i].WeekRef.StartDate.Before(results[j].WeekRef.StartDate)
		}
		return results[i].User.FullName < results[j].User.FullName
	})

	first, last := weeks[0], weeks[len(weeks)-1]
	return domain.TimeStatusReport{From: first, To: last, Rows: results}, nil
}

// resolveWeeks converts statusFlags' --week / --from/--to into a list of
// WeekRefs (Sunday→Saturday in EasternTZ).
func resolveWeeks(f statusFlags) ([]domain.WeekRef, error) {
	if f.week != "" {
		t, err := time.ParseInLocation("2006-01-02", f.week, domain.EasternTZ)
		if err != nil {
			return nil, fmt.Errorf("invalid --week: %w", err)
		}
		return []domain.WeekRef{domain.WeekRefContaining(t)}, nil
	}
	from, err := time.ParseInLocation("2006-01-02", f.from, domain.EasternTZ)
	if err != nil {
		return nil, fmt.Errorf("invalid --from: %w", err)
	}
	to, err := time.ParseInLocation("2006-01-02", f.to, domain.EasternTZ)
	if err != nil {
		return nil, fmt.Errorf("invalid --to: %w", err)
	}
	if to.Before(from) {
		return nil, fmt.Errorf("--to (%s) before --from (%s)", to.Format("2006-01-02"), from.Format("2006-01-02"))
	}
	startWeek := domain.WeekRefContaining(from)
	endWeek := domain.WeekRefContaining(to)
	weeks := []domain.WeekRef{}
	cur := startWeek
	for !cur.StartDate.After(endWeek.StartDate) {
		weeks = append(weeks, cur)
		cur = domain.WeekRefContaining(cur.StartDate.AddDate(0, 0, 7))
	}
	return weeks, nil
}

// resolveUsers maps the selector flags to a concrete user list.
// Pre-validated: exactly one of --user/--manager/--account/--all is set.
func resolveUsers(ctx context.Context, deps runnerDeps, f statusFlags) ([]domain.User, error) {
	switch {
	case len(f.users) > 0:
		out := make([]domain.User, 0, len(f.users))
		for _, uid := range f.users {
			u, err := deps.People.GetUser(ctx, deps.Profile, uid)
			if err != nil {
				return nil, fmt.Errorf("get user %s: %w", uid, err)
			}
			out = append(out, u)
		}
		return out, nil

	case f.manager != "":
		mgrUID := f.manager
		if mgrUID == "me" {
			me, err := deps.Auth.WhoAmI(ctx, deps.Profile)
			if err != nil {
				return nil, fmt.Errorf("whoami: %w", err)
			}
			mgrUID = me.UID
		}
		all, err := deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{Limit: hardLimit})
		if err != nil {
			return nil, err
		}
		out := []domain.User{}
		for _, u := range all {
			if u.ReportsToUID == mgrUID {
				out = append(out, u)
			}
		}
		return out, nil

	case f.account != "":
		return deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
			AccountName: f.account,
			Limit:       hardLimit,
		})

	case f.all:
		return deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{Limit: hardLimit})
	}
	return nil, fmt.Errorf("no selector (validation should have caught this)")
}
