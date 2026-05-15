package report

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"golang.org/x/sync/errgroup"
)

// timesvcEntriesAPI is the subset of timesvc for SearchEntries (project post-filter).
type timesvcEntriesAPI interface {
	SearchEntries(ctx context.Context, profile string, filter domain.EntryFilter) ([]domain.TimeEntry, error)
}

// reportProjectsvcAPI is the subset of projectsvc the runner needs for --project.
type reportProjectsvcAPI interface {
	ListResources(ctx context.Context, profile string, projectID int) ([]domain.ProjectResource, error)
}

// timesvcAPI is the subset of timesvc.Service the runner needs.
type timesvcAPI interface {
	GetWeekReportForUser(ctx context.Context, profile string, date time.Time, uid string) (domain.WeekReport, error)
}

// peoplesvcAPI is the subset of peoplesvc.Service the runner needs.
type peoplesvcAPI interface {
	GetUser(ctx context.Context, profile, uid string) (domain.User, error)
	SearchUsers(ctx context.Context, profile string, filter domain.UserFilter) ([]domain.User, error)
	ResolvePoolByName(ctx context.Context, profile, name string) (peoplesvc.ResourcePool, error)
	ResolveAccountByName(ctx context.Context, profile, name string) (peoplesvc.Account, error)
}

// authsvcAPI is the subset of authsvc.Service the runner needs.
type authsvcAPI interface {
	WhoAmI(ctx context.Context, profile string) (domain.User, error)
}

// runnerDeps bundles the service dependencies for assembleReport so tests
// can inject mocks via the interfaces above.
type runnerDeps struct {
	Time    timesvcAPI
	Entries timesvcEntriesAPI
	People  peoplesvcAPI
	Auth    authsvcAPI
	Project reportProjectsvcAPI
	Profile string
}

const (
	maxConcurrency = 5
	// employeeLimit caps the people search when filtering by IsEmployee.
	// Tenants commonly have 1000-2000 employees; 5000 is well under TD's 10K behavior cap.
	employeeLimit = 5000
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

	// Refuse if resolved user set exceeds the hard cap. This catches wide-
	// selector cases (e.g. --all on a multi-thousand-staff tenant) that today
	// would silently truncate. --limit N (user-explicit narrowing where
	// N ≤ MaxReportUsers) is checked separately in validateStatusFlags.
	if len(users) > domain.MaxReportUsers {
		return domain.TimeStatusReport{}, fmt.Errorf("%w: users=%d max=%d; narrow with --resource-pool, --account, or --manager",
			domain.ErrFanoutLimitExceeded, len(users), domain.MaxReportUsers)
	}

	// Apply --limit cap (user-explicit narrowing).
	cap := f.limit
	if cap <= 0 {
		cap = domain.MaxReportUsers
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
				row := domain.WeekStatusRow{
					WeekRef: week,
					User:    u,
				}
				if f.projectID > 0 && deps.Entries != nil {
					// Project-scoped mode: fetch the user's project entries
					// directly and sum. Skips GetWeekReportForUser to keep API
					// call count to N (not 2N) — important for big projects
					// where TD's 60/min/IP rate limit becomes a hazard.
					// Trade-off: row.Status is left empty (week submission
					// status is per-user-week, not per-project, so it's not
					// meaningful in this mode).
					rng := domain.DateRange{From: week.StartDate, To: week.EndDate}
					entries, err := deps.Entries.SearchEntries(gctx, deps.Profile, domain.EntryFilter{
						DateRange: rng,
						UserUID:   u.UID,
						ProjectID: f.projectID,
					})
					switch {
					case err == nil:
						var bill, nonBill, total int
						for _, e := range entries {
							total += e.Minutes
							if e.Billable {
								bill += e.Minutes
							} else {
								nonBill += e.Minutes
							}
						}
						row.BillableMin = bill
						row.NonBillableMin = nonBill
						row.TotalMin = total
					case errors.Is(err, domain.ErrPermission):
						row.Status = domain.ReportStatus("permission-denied")
					default:
						return fmt.Errorf("search entries for %s/%s: %w", u.UID, week.StartDate.Format("2006-01-02"), err)
					}
				} else {
					rep, err := deps.Time.GetWeekReportForUser(gctx, deps.Profile, week.StartDate, u.UID)
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

	// Apply --incomplete filter (keeps rows below threshold; drops
	// permission-denied since we can't classify hours we couldn't read).
	// Threshold mode:
	//   - thresholdSet=true   → global f.threshold (or 40 if <= 0) for every row
	//   - thresholdSet=false  → per-user from row.User.WorkableHours * workdaysPerWeek
	//                           (TD's WorkableHours is per-DAY, e.g. 8.0 for FT);
	//                           falls back to defaultThresholdFallback (40) when 0
	const defaultThresholdFallback = 40.0
	const workdaysPerWeek = 5.0
	if f.incomplete {
		globalThreshold := f.threshold
		if globalThreshold <= 0 {
			globalThreshold = defaultThresholdFallback
		}
		filtered := results[:0]
		for _, r := range results {
			if r.Status == domain.ReportStatus("permission-denied") {
				continue
			}
			var rowThreshold float64
			if f.thresholdSet {
				rowThreshold = globalThreshold
			} else if r.User.WorkableHours > 0 {
				rowThreshold = r.User.WorkableHours * workdaysPerWeek
			} else {
				rowThreshold = defaultThresholdFallback
			}
			r.Threshold = rowThreshold // set on the local copy before the skip check
			if r.TotalHours() >= rowThreshold {
				continue
			}
			filtered = append(filtered, r)
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

// MCPInputs is the input for RunForMCP. Mirrors the CLI flags but exposed
// as a typed Go struct for use by the MCP handler.
type MCPInputs struct {
	Profile       string
	Week          string
	From, To      string
	Users         []string
	Managers      []string
	Accounts      []string
	ResourcePools []string
	All           bool
	ProjectID     int
	IncludeZero   bool
	Incomplete    bool
	Threshold     float64
	ThresholdSet  bool // mirror of statusFlags.thresholdSet — populated by handler from `Threshold > 0`
	Limit         int
	TimeSvc       timesvcAPI
	EntriesSvc    timesvcEntriesAPI
	PeopleSvc     peoplesvcAPI
	AuthSvc       authsvcAPI
	ProjectSvc    reportProjectsvcAPI
}

// RunForMCP builds, validates, and runs a Time Status Report for MCP
// callers. Returns the JSON-shaped envelope (same shape as printJSON's
// output) for direct marshaling. Bypasses --yes for --all (the agent
// has already opted in).
func RunForMCP(ctx context.Context, in MCPInputs) (any, error) {
	f := statusFlags{
		profile:       in.Profile,
		week:          in.Week,
		from:          in.From,
		to:            in.To,
		users:         in.Users,
		managers:      in.Managers,
		accounts:      in.Accounts,
		resourcePools: in.ResourcePools,
		all:           in.All,
		yes:           in.All, // bypass --yes guard for MCP
		projectID:     in.ProjectID,
		includeZero:   in.IncludeZero,
		incomplete:    in.Incomplete,
		threshold:     in.Threshold,
		thresholdSet:  in.ThresholdSet,
		limit:         in.Limit,
	}
	if err := validateStatusFlags(f); err != nil {
		return nil, err
	}
	deps := runnerDeps{
		Time:    in.TimeSvc,
		Entries: in.EntriesSvc,
		People:  in.PeopleSvc,
		Auth:    in.AuthSvc,
		Project: in.ProjectSvc,
		Profile: in.Profile,
	}
	rep, err := assembleReport(ctx, deps, f)
	if err != nil {
		return nil, err
	}
	return buildJSONEnvelope(rep, f), nil
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
	if span := domain.WeekSpan(from, to); span > domain.MaxReportWeeks {
		return nil, fmt.Errorf("%w: weeks=%d max=%d; narrow the --from/--to range",
			domain.ErrFanoutLimitExceeded, span, domain.MaxReportWeeks)
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
// Pre-validated: exactly one selector type is in use; each type may carry
// multiple values that are unioned.
func resolveUsers(ctx context.Context, deps runnerDeps, f statusFlags) ([]domain.User, error) {
	trueVal := true
	switch {
	case len(f.users) > 0:
		seen := map[string]struct{}{}
		out := make([]domain.User, 0, len(f.users))
		for _, uid := range f.users {
			if _, dup := seen[uid]; dup {
				continue
			}
			seen[uid] = struct{}{}
			u, err := deps.People.GetUser(ctx, deps.Profile, uid)
			if err != nil {
				return nil, fmt.Errorf("get user %s: %w", uid, err)
			}
			out = append(out, u)
		}
		return out, nil

	case len(f.managers) > 0:
		mgrUIDs := map[string]struct{}{}
		for _, raw := range f.managers {
			uid := raw
			if uid == "me" {
				me, err := deps.Auth.WhoAmI(ctx, deps.Profile)
				if err != nil {
					return nil, fmt.Errorf("whoami: %w", err)
				}
				uid = me.UID
			}
			mgrUIDs[uid] = struct{}{}
		}
		all, err := deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
			Employee: &trueVal,
			Limit:    employeeLimit,
		})
		if err != nil {
			return nil, err
		}
		out := []domain.User{}
		for _, u := range all {
			if _, ok := mgrUIDs[u.ReportsToUID]; ok {
				out = append(out, u)
			}
		}
		return out, nil

	case len(f.accounts) > 0:
		ids := make([]int, 0, len(f.accounts))
		seen := map[int]struct{}{}
		for _, name := range f.accounts {
			acct, err := deps.People.ResolveAccountByName(ctx, deps.Profile, name)
			if err != nil {
				return nil, err
			}
			if _, dup := seen[acct.ID]; dup {
				continue
			}
			seen[acct.ID] = struct{}{}
			ids = append(ids, acct.ID)
		}
		return deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
			Employee:   &trueVal,
			AccountIDs: ids,
			Limit:      employeeLimit,
		})

	case f.all:
		return deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
			Employee: &trueVal,
			Limit:    employeeLimit,
		})

	case len(f.resourcePools) > 0:
		poolIDs := map[int]struct{}{}
		for _, name := range f.resourcePools {
			pool, err := deps.People.ResolvePoolByName(ctx, deps.Profile, name)
			if err != nil {
				return nil, err
			}
			poolIDs[pool.ID] = struct{}{}
		}
		all, err := deps.People.SearchUsers(ctx, deps.Profile, domain.UserFilter{
			Employee: &trueVal,
			Limit:    employeeLimit,
		})
		if err != nil {
			return nil, err
		}
		out := []domain.User{}
		for _, u := range all {
			if _, ok := poolIDs[u.ResourcePoolID]; ok {
				out = append(out, u)
			}
		}
		return out, nil

	case f.projectID > 0:
		if deps.Project == nil {
			return nil, fmt.Errorf("project service not configured (internal error)")
		}
		resources, err := deps.Project.ListResources(ctx, deps.Profile, f.projectID)
		if err != nil {
			return nil, fmt.Errorf("list project %d resources: %w", f.projectID, err)
		}
		out := make([]domain.User, 0, len(resources))
		for _, r := range resources {
			out = append(out, domain.User{UID: r.UID, FullName: r.FullName})
		}
		return out, nil
	}
	return nil, fmt.Errorf("no selector (validation should have caught this)")
}
