package project

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/render"
	"github.com/iainmoffat/tdx/internal/svc/authsvc"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
	"github.com/iainmoffat/tdx/internal/svc/projectsvc"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

// timesvcTimeAPI is the subset of timesvc the project-time command needs.
type timesvcTimeAPI interface {
	SearchEntries(ctx context.Context, profile string, filter domain.EntryFilter) ([]domain.TimeEntry, error)
}

const projectTimeFanoutConcurrency = 5

func newTimeCmd(psvc projectsvcAPI, tsvc timesvcTimeAPI) *cobra.Command {
	var (
		weekFlag     string
		fromFlag     string
		toFlag       string
		userFlags    []string
		allUsersFlag bool
		planFlag     int
		taskFlag     int
		limitFlag    int
		jsonFlag     bool
		profileFlag  string
	)

	cmd := &cobra.Command{
		Use:   "time <project-id>",
		Short: "Show time entries logged against a project",
		Long: `Show time entries logged against a project, scoped to one or more users and a date range.

Defaults to the authenticated user's entries for the current week.
Use --all-users to show entries for all project team members.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Pre-config validation (before config.ResolvePaths).
			if len(args) == 0 {
				return fmt.Errorf("project-id is required")
			}
			projectID, err := strconv.Atoi(args[0])
			if err != nil || projectID <= 0 {
				return fmt.Errorf("project id must be a positive integer, got %q", args[0])
			}
			if allUsersFlag && len(userFlags) > 0 {
				return fmt.Errorf("--user and --all-users are mutually exclusive")
			}
			if weekFlag != "" && (fromFlag != "" || toFlag != "") {
				return fmt.Errorf("--week is mutually exclusive with --from/--to")
			}
			if (fromFlag != "") != (toFlag != "") {
				return fmt.Errorf("--from and --to must be given together")
			}

			paths, err := config.ResolvePaths()
			if err != nil {
				return err
			}
			auth := authsvc.New(paths)
			profile, err := auth.ResolveProfile(profileFlag)
			if err != nil {
				return err
			}

			// Resolve date range.
			var rng domain.DateRange
			switch {
			case weekFlag != "":
				day, err := time.ParseInLocation("2006-01-02", weekFlag, domain.EasternTZ)
				if err != nil {
					return fmt.Errorf("invalid --week: %w", err)
				}
				w := domain.WeekRefContaining(day)
				rng = domain.DateRange{From: w.StartDate, To: w.EndDate}
			case fromFlag != "":
				from, err := time.ParseInLocation("2006-01-02", fromFlag, domain.EasternTZ)
				if err != nil {
					return fmt.Errorf("invalid --from: %w", err)
				}
				to, err := time.ParseInLocation("2006-01-02", toFlag, domain.EasternTZ)
				if err != nil {
					return fmt.Errorf("invalid --to: %w", err)
				}
				rng = domain.DateRange{From: from, To: to}
			default:
				// Default: current week.
				w := domain.WeekRefContaining(time.Now())
				rng = domain.DateRange{From: w.StartDate, To: w.EndDate}
			}

			// Resolve services.
			ps := psvc
			if ps == nil {
				ps = projectsvc.New(paths)
			}
			ts := tsvc
			if ts == nil {
				ts = timesvc.New(paths)
			}

			// Resolve users.
			authedUID, err := authedUIDFor(cmd.Context(), auth, profile)
			if err != nil {
				return err
			}

			var users []domain.User
			switch {
			case allUsersFlag:
				resources, err := ps.ListResources(cmd.Context(), profile, projectID)
				if err != nil {
					return fmt.Errorf("list project resources: %w", err)
				}
				users = make([]domain.User, 0, len(resources))
				for _, r := range resources {
					users = append(users, domain.User{UID: r.UID, FullName: r.FullName})
				}
			case len(userFlags) > 0:
				peopleSvc := peoplesvc.New(paths)
				users = make([]domain.User, 0, len(userFlags))
				for _, arg := range userFlags {
					uid, err := resolvePrincipal(cmd.Context(), peopleSvc, profile, authedUID, arg)
					if err != nil {
						return fmt.Errorf("resolve user %q: %w", arg, err)
					}
					u, err := peopleSvc.GetUser(cmd.Context(), profile, uid)
					if err != nil {
						// Fall back to UID-only user if GetUser fails.
						u = domain.User{UID: uid}
					}
					users = append(users, u)
				}
			default:
				// Default: me.
				me, err := auth.WhoAmI(cmd.Context(), profile)
				if err != nil {
					return fmt.Errorf("whoami: %w", err)
				}
				users = []domain.User{{UID: me.UID, FullName: me.FullName}}
			}

			return runProjectTimeRender(cmd.Context(), cmd.OutOrStdout(), ts, profile,
				projectID, planFlag, taskFlag, rng, users, jsonFlag)

		},
	}

	cmd.Flags().StringVar(&weekFlag, "week", "", "any date inside the target week (YYYY-MM-DD)")
	cmd.Flags().StringVar(&fromFlag, "from", "", "range start (YYYY-MM-DD); requires --to")
	cmd.Flags().StringVar(&toFlag, "to", "", "range end (YYYY-MM-DD); requires --from")
	cmd.Flags().StringArrayVar(&userFlags, "user", nil, "user UID/email/\"me\" (repeatable; default: me)")
	cmd.Flags().BoolVar(&allUsersFlag, "all-users", false, "all project team members (mutually exclusive with --user)")
	cmd.Flags().IntVar(&planFlag, "plan", 0, "narrow to a specific plan ID")
	cmd.Flags().IntVar(&taskFlag, "task", 0, "narrow to a specific task ID")
	cmd.Flags().IntVar(&limitFlag, "limit", 0, "maximum entries per user (0 = no limit)")
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "emit JSON output")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile name (defaults to active profile)")
	return cmd
}

// runProjectTimeRender fetches entries for each user and renders the result.
// Extracted for testability.
func runProjectTimeRender(
	ctx context.Context,
	w io.Writer,
	ts timesvcTimeAPI,
	profile string,
	projectID, planID, taskID int,
	rng domain.DateRange,
	users []domain.User,
	jsonOut bool,
) error {
	// Fan-out: fetch entries per user.
	mu := sync.Mutex{}
	allEntries := make([]domain.TimeEntry, 0, len(users)*10)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(projectTimeFanoutConcurrency)

	for _, u := range users {
		u := u
		g.Go(func() error {
			filter := domain.EntryFilter{
				DateRange: rng,
				UserUID:   u.UID,
				ProjectID: projectID,
				PlanID:    planID,
				TaskID:    taskID,
			}
			entries, err := ts.SearchEntries(gctx, profile, filter)
			if err != nil {
				return fmt.Errorf("search entries for %s: %w", u.UID, err)
			}
			mu.Lock()
			allEntries = append(allEntries, entries...)
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Sort by date asc, then by userUID.
	sort.SliceStable(allEntries, func(i, j int) bool {
		if !allEntries[i].Date.Equal(allEntries[j].Date) {
			return allEntries[i].Date.Before(allEntries[j].Date)
		}
		return allEntries[i].UserUID < allEntries[j].UserUID
	})

	// Build UID → fullName map.
	uidToName := make(map[string]string, len(users))
	for _, u := range users {
		uidToName[u.UID] = u.FullName
	}

	// Compute totals.
	totalMin := 0
	billableMin := 0
	nonBillableMin := 0
	perUser := make(map[string]int, len(users))
	for _, e := range allEntries {
		totalMin += e.Minutes
		if e.Billable {
			billableMin += e.Minutes
		} else {
			nonBillableMin += e.Minutes
		}
		perUser[e.UserUID] += e.Minutes
	}

	if jsonOut {
		type userJSON struct {
			UID      string `json:"uid"`
			FullName string `json:"fullName,omitempty"`
		}
		type entryJSON struct {
			ID           int     `json:"id"`
			Date         string  `json:"date"`
			UserUID      string  `json:"userUID"`
			UserFullName string  `json:"userFullName,omitempty"`
			TypeName     string  `json:"typeName,omitempty"`
			Kind         string  `json:"kind"`
			ProjectID    int     `json:"projectID,omitempty"`
			PlanID       int     `json:"planID,omitempty"`
			TaskID       int     `json:"taskID,omitempty"`
			Hours        float64 `json:"hours"`
			Billable     bool    `json:"billable"`
			Description  string  `json:"description,omitempty"`
		}
		usersJSON := make([]userJSON, 0, len(users))
		for _, u := range users {
			usersJSON = append(usersJSON, userJSON{UID: u.UID, FullName: u.FullName})
		}
		entriesJSON := make([]entryJSON, 0, len(allEntries))
		for _, e := range allEntries {
			ej := entryJSON{
				ID:           e.ID,
				Date:         e.Date.Format("2006-01-02"),
				UserUID:      e.UserUID,
				UserFullName: uidToName[e.UserUID],
				TypeName:     e.TimeType.Name,
				Kind:         string(e.Target.Kind),
				Hours:        e.Hours(),
				Billable:     e.Billable,
				Description:  e.Description,
			}
			switch e.Target.Kind {
			case domain.TargetProject:
				ej.ProjectID = e.Target.ItemID
			case domain.TargetProjectTask:
				ej.ProjectID = e.Target.ProjectID
				ej.PlanID = e.Target.ItemID
				ej.TaskID = e.Target.TaskID
			case domain.TargetProjectIssue:
				ej.ProjectID = e.Target.ProjectID
			}
			entriesJSON = append(entriesJSON, ej)
		}
		return render.JSON(w, struct {
			Schema           string      `json:"schema"`
			ProjectID        int         `json:"projectID"`
			DateRange        any         `json:"dateRange"`
			Users            []userJSON  `json:"users"`
			TotalHours       float64     `json:"totalHours"`
			BillableHours    float64     `json:"billableHours"`
			NonBillableHours float64     `json:"nonBillableHours"`
			Entries          []entryJSON `json:"entries"`
		}{
			Schema:    "tdx.v1.projectTimeReview",
			ProjectID: projectID,
			DateRange: map[string]string{
				"from": rng.From.Format("2006-01-02"),
				"to":   rng.To.Format("2006-01-02"),
			},
			Users:            usersJSON,
			TotalHours:       float64(totalMin) / 60.0,
			BillableHours:    float64(billableMin) / 60.0,
			NonBillableHours: float64(nonBillableMin) / 60.0,
			Entries:          entriesJSON,
		})
	}

	// Human output.
	headers := []string{"DATE", "USER", "TYPE", "KIND", "REF", "HOURS", "DESCRIPTION"}
	rows := make([][]string, 0, len(allEntries))
	for _, e := range allEntries {
		name := uidToName[e.UserUID]
		if name == "" {
			name = e.UserUID
		}
		rows = append(rows, []string{
			e.Date.Format("2006-01-02"),
			truncate(name, 20),
			e.TimeType.Name,
			string(e.Target.Kind),
			e.Target.DisplayRef,
			fmt.Sprintf("%.2f", e.Hours()),
			truncate(e.Description, 60),
		})
	}
	summary := []string{"TOTAL", "", "", "", "", fmt.Sprintf("%.2f", float64(totalMin)/60.0), ""}
	render.Table(w, headers, rows, summary)

	// Per-user footer if multi-user.
	if len(users) > 1 {
		_, _ = fmt.Fprintln(w)
		for _, u := range users {
			name := u.FullName
			if name == "" {
				name = u.UID
			}
			_, _ = fmt.Fprintf(w, "  %-30s %.2fh\n", truncate(name, 30), float64(perUser[u.UID])/60.0)
		}
	}
	return nil
}
