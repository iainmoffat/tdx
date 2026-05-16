package mcp

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/errgroup"

	"github.com/iainmoffat/tdx/internal/domain"
)

// --- Argument types ---

type getProjectTimeReviewArgs struct {
	Profile   string   `json:"profile,omitempty"`
	ProjectID int      `json:"projectID"`
	Week      string   `json:"week,omitempty"`
	From      string   `json:"from,omitempty"`
	To        string   `json:"to,omitempty"`
	UserUIDs  []string `json:"userUIDs,omitempty"`
	AllUsers  bool     `json:"allUsers,omitempty"`
	PlanID    int      `json:"planID,omitempty"`
	TaskID    int      `json:"taskID,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

type listMyProjectsArgs struct {
	Profile string `json:"profile,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

type searchProjectsArgs struct {
	Profile    string `json:"profile,omitempty"`
	Query      string `json:"query,omitempty"`
	ManagerUID string `json:"managerUID,omitempty"`
	StatusIDs  []int  `json:"statusIDs,omitempty"`
	TypeIDs    []int  `json:"typeIDs,omitempty"`
	IsActive   *bool  `json:"isActive,omitempty"`
	IsOpen     *bool  `json:"isOpen,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type getProjectArgs struct {
	Profile string `json:"profile,omitempty"`
	ID      int    `json:"id"`
}

type listProjectPlansArgs struct {
	Profile      string `json:"profile,omitempty"`
	ProjectID    int    `json:"projectID"`
	NameLike     string `json:"nameLike,omitempty"`
	IncludeEmpty bool   `json:"includeEmpty,omitempty"`
}

type listProjectTasksArgs struct {
	Profile   string `json:"profile,omitempty"`
	ProjectID int    `json:"projectID,omitempty"`
	PlanID    int    `json:"planID,omitempty"`
	Mine      bool   `json:"mine,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type getProjectTaskArgs struct {
	Profile   string `json:"profile,omitempty"`
	ProjectID int    `json:"projectID"`
	PlanID    int    `json:"planID"`
	TaskID    int    `json:"taskID"`
}

type getProjectFeedArgs struct {
	Profile   string `json:"profile,omitempty"`
	ProjectID int    `json:"projectID"`
	Limit     int    `json:"limit,omitempty"`
}

type getProjectTaskFeedArgs struct {
	Profile   string `json:"profile,omitempty"`
	ProjectID int    `json:"projectID"`
	PlanID    int    `json:"planID"`
	TaskID    int    `json:"taskID"`
	Limit     int    `json:"limit,omitempty"`
}

type projectFeedEntryJSON struct {
	ID            int    `json:"id"`
	CreatedByUID  string `json:"createdByUID,omitempty"`
	CreatedByName string `json:"createdByName,omitempty"`
	CreatedDate   string `json:"createdDate,omitempty"`
	UpdateType    string `json:"updateType,omitempty"`
	Body          string `json:"body"`
	IsPrivate     bool   `json:"isPrivate"`
	LikesCount    int    `json:"likesCount,omitempty"`
	RepliesCount  int    `json:"repliesCount,omitempty"`
}

// --- JSON row types ---

type projectPlanRowJSON struct {
	PlanID          int     `json:"planID"`
	ProjectID       int     `json:"projectID"`
	ProjectName     string  `json:"projectName"`
	Title           string  `json:"title"`
	Type            string  `json:"type"`
	MyTaskCount     int     `json:"myTaskCount,omitempty"`
	TaskCount       int     `json:"taskCount,omitempty"`
	PercentComplete float64 `json:"percentComplete,omitempty"`
	StartDate       string  `json:"startDate,omitempty"`
	EndDate         string  `json:"endDate,omitempty"`
}

type projectDetailJSON struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	StatusName      string  `json:"statusName,omitempty"`
	TypeName        string  `json:"typeName,omitempty"`
	AccountName     string  `json:"accountName,omitempty"`
	ManagerUID      string  `json:"managerUID,omitempty"`
	ManagerName     string  `json:"managerName,omitempty"`
	SponsorName     string  `json:"sponsorName,omitempty"`
	PercentComplete float64 `json:"percentComplete,omitempty"`
	EstimatedHours  float64 `json:"estimatedHours,omitempty"`
	ActualHours     float64 `json:"actualHours,omitempty"`
	StartDate       string  `json:"startDate,omitempty"`
	EndDate         string  `json:"endDate,omitempty"`
	ModifiedDate    string  `json:"modifiedDate,omitempty"`
	IsActive        bool    `json:"isActive,omitempty"`
}

type projectTaskRowJSON struct {
	ProjectID       int     `json:"projectID"`
	PlanID          int     `json:"planID"`
	TaskID          int     `json:"taskID"`
	Title           string  `json:"title"`
	Status          string  `json:"status,omitempty"`
	PercentComplete float64 `json:"percentComplete,omitempty"`
	EstimatedHours  float64 `json:"estimatedHours,omitempty"`
	ActualHours     float64 `json:"actualHours,omitempty"`
	EndDate         string  `json:"endDate,omitempty"`
}

type projectTaskResourceJSON struct {
	UID             string  `json:"uid"`
	FullName        string  `json:"fullName"`
	RoleName        string  `json:"roleName,omitempty"`
	PercentAssigned float64 `json:"percentAssigned,omitempty"`
}

type projectTaskDetailJSON struct {
	ProjectID       int                       `json:"projectID"`
	PlanID          int                       `json:"planID"`
	TaskID          int                       `json:"taskID"`
	Title           string                    `json:"title"`
	Status          string                    `json:"status,omitempty"`
	PercentComplete float64                   `json:"percentComplete,omitempty"`
	EstimatedHours  float64                   `json:"estimatedHours,omitempty"`
	ActualHours     float64                   `json:"actualHours,omitempty"`
	RemainingHours  float64                   `json:"remainingHours,omitempty"`
	StartDate       string                    `json:"startDate,omitempty"`
	EndDate         string                    `json:"endDate,omitempty"`
	IsParent        bool                      `json:"isParent,omitempty"`
	OutlineNumber   string                    `json:"outlineNumber,omitempty"`
	Description     string                    `json:"description,omitempty"`
	Resources       []projectTaskResourceJSON `json:"resources,omitempty"`
}

func toProjectPlanRowJSON(p domain.ProjectPlan) projectPlanRowJSON {
	return projectPlanRowJSON{
		PlanID: p.ID, ProjectID: p.ProjectID, ProjectName: p.ProjectName,
		Title: p.Title, Type: p.Type.String(),
		MyTaskCount: p.MyTaskCount, TaskCount: p.TaskCount,
		PercentComplete: p.PercentComplete,
		StartDate:       formatProjectDate(p.StartDate),
		EndDate:         formatProjectDate(p.EndDate),
	}
}

func toProjectDetailJSON(p domain.Project) projectDetailJSON {
	return projectDetailJSON{
		ID:              p.ID,
		Name:            p.Name,
		StatusName:      p.StatusName,
		TypeName:        p.TypeName,
		AccountName:     p.AccountName,
		ManagerUID:      p.ManagerUID,
		ManagerName:     p.ManagerName,
		SponsorName:     p.SponsorName,
		PercentComplete: p.PercentComplete,
		EstimatedHours:  p.EstimatedHours,
		ActualHours:     p.ActualHours,
		StartDate:       formatProjectDate(p.StartDate),
		EndDate:         formatProjectDate(p.EndDate),
		ModifiedDate:    formatProjectDate(p.ModifiedDate),
		IsActive:        p.IsActive,
	}
}

func toProjectTaskRowJSON(t domain.ProjectTask) projectTaskRowJSON {
	return projectTaskRowJSON{
		ProjectID: t.ProjectID, PlanID: t.PlanID, TaskID: t.ID,
		Title: t.Title, Status: t.Status,
		PercentComplete: t.PercentComplete,
		EstimatedHours:  t.EstimatedHours,
		ActualHours:     t.ActualHours,
		EndDate:         formatProjectDate(t.EndDate),
	}
}

func toProjectTaskDetailJSON(t domain.ProjectTask) projectTaskDetailJSON {
	resources := make([]projectTaskResourceJSON, 0, len(t.Resources))
	for _, r := range t.Resources {
		resources = append(resources, projectTaskResourceJSON{
			UID:             r.UID,
			FullName:        r.FullName,
			RoleName:        r.RoleName,
			PercentAssigned: r.PercentAssigned,
		})
	}
	return projectTaskDetailJSON{
		ProjectID: t.ProjectID, PlanID: t.PlanID, TaskID: t.ID,
		Title: t.Title, Status: t.Status,
		PercentComplete: t.PercentComplete,
		EstimatedHours:  t.EstimatedHours,
		ActualHours:     t.ActualHours,
		RemainingHours:  t.RemainingHours,
		StartDate:       formatProjectDate(t.StartDate),
		EndDate:         formatProjectDate(t.EndDate),
		IsParent:        t.IsParent,
		OutlineNumber:   t.OutlineNumber,
		Description:     t.Description,
		Resources:       resources,
	}
}

func formatProjectDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// RegisterProjectTools registers the 9 read-only project MCP tools.
func RegisterProjectTools(srv *sdkmcp.Server, svcs Services) {
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_project_time_review",
		Description: "Show time entries logged against a project for one or more users in a date range. Defaults to the authenticated user's entries for the current week. Use allUsers=true to show entries for all project team members (mutually exclusive with userUIDs). Read-only.",
	}, getProjectTimeReviewHandler(svcs))
	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_my_projects",
		Description: "List projects and plans the authenticated user participates in. Read-only.",
	}, listMyProjectsHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "search_projects",
		Description: "Search for projects by name and filters (isActive, managerUID, statusIDs, typeIDs). Read-only.",
	}, searchProjectsHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_project",
		Description: "Get full detail for one project by ID. Read-only.",
	}, getProjectHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_project_plans",
		Description: "List plans for a project. Read-only.",
	}, listProjectPlansHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "list_project_tasks",
		Description: "List project tasks. Provide projectID+planID for single-plan mode, or mine=true for cross-project tasks assigned to the authenticated user. Read-only.",
	}, listProjectTasksHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_project_task",
		Description: "Get full detail for one project task. Read-only.",
	}, getProjectTaskHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_project_feed",
		Description: "Get the feed (activity + comments) for a project. Read-only.",
	}, getProjectFeedHandler(svcs))

	sdkmcp.AddTool(srv, &sdkmcp.Tool{
		Name:        "get_project_task_feed",
		Description: "Get the feed (activity + comments) for a project task. Read-only.",
	}, getProjectTaskFeedHandler(svcs))
}

// --- Handlers ---

func listMyProjectsHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listMyProjectsArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listMyProjectsArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		plans, err := svcs.Projects.ListMine(ctx, profile)
		if err != nil {
			return errorResult("list_my_projects: " + err.Error()), nil, nil
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 50
		}
		if len(plans) > limit {
			plans = plans[:limit]
		}
		out := make([]projectPlanRowJSON, 0, len(plans))
		for _, p := range plans {
			out = append(out, toProjectPlanRowJSON(p))
		}
		return jsonResult(struct {
			Schema string               `json:"schema"`
			Plans  []projectPlanRowJSON `json:"plans"`
		}{Schema: "tdx.v1.projectPlanList", Plans: out})
	}
}

func searchProjectsHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, searchProjectsArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args searchProjectsArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		limit := args.Limit
		if limit <= 0 {
			limit = 50
		}
		filter := domain.ProjectSearchFilter{
			NameLike:   args.Query,
			ManagerUID: args.ManagerUID,
			StatusIDs:  args.StatusIDs,
			TypeIDs:    args.TypeIDs,
			IsActive:   args.IsActive,
			IsOpen:     args.IsOpen,
			MaxResults: limit,
		}
		projects, err := svcs.Projects.Search(ctx, profile, filter)
		if err != nil {
			return errorResult("search_projects: " + err.Error()), nil, nil
		}
		out := make([]projectDetailJSON, 0, len(projects))
		for _, p := range projects {
			out = append(out, toProjectDetailJSON(p))
		}
		return jsonResult(struct {
			Schema   string              `json:"schema"`
			Projects []projectDetailJSON `json:"projects"`
		}{Schema: "tdx.v1.projectList", Projects: out})
	}
}

func getProjectHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getProjectArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getProjectArgs) (*sdkmcp.CallToolResult, any, error) {
		if args.ID <= 0 {
			return errorResult("get_project: id is required"), nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		p, err := svcs.Projects.Get(ctx, profile, args.ID)
		if err != nil {
			return errorResult("get_project: " + err.Error()), nil, nil
		}
		return jsonResult(struct {
			Schema  string            `json:"schema"`
			Project projectDetailJSON `json:"project"`
		}{Schema: "tdx.v1.project", Project: toProjectDetailJSON(p)})
	}
}

func listProjectPlansHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listProjectPlansArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listProjectPlansArgs) (*sdkmcp.CallToolResult, any, error) {
		if args.ProjectID <= 0 {
			return errorResult("list_project_plans: projectID is required"), nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		plans, err := svcs.Projects.SearchPlans(ctx, profile, args.ProjectID, args.NameLike, args.IncludeEmpty)
		if err != nil {
			return errorResult("list_project_plans: " + err.Error()), nil, nil
		}
		out := make([]projectPlanRowJSON, 0, len(plans))
		for _, p := range plans {
			out = append(out, toProjectPlanRowJSON(p))
		}
		return jsonResult(struct {
			Schema string               `json:"schema"`
			Plans  []projectPlanRowJSON `json:"plans"`
		}{Schema: "tdx.v1.projectPlanList", Plans: out})
	}
}

func listProjectTasksHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, listProjectTasksArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args listProjectTasksArgs) (*sdkmcp.CallToolResult, any, error) {
		profile := resolveProfile(svcs, args.Profile)
		limit := args.Limit
		if limit <= 0 {
			limit = 50
		}

		// Validate: mine XOR (projectID+planID).
		if args.Mine && (args.ProjectID > 0 || args.PlanID > 0) {
			return errorResult("list_project_tasks: mine=true is mutually exclusive with projectID and planID"), nil, nil
		}
		if !args.Mine && (args.ProjectID <= 0 || args.PlanID <= 0) {
			return errorResult("list_project_tasks: provide projectID+planID for single-plan mode, or mine=true for cross-project tasks"), nil, nil
		}

		var tasks []domain.ProjectTask
		if args.Mine {
			// --mine fanout: resolve authed UID, then fan out.
			user, err := svcs.Auth.WhoAmI(ctx, profile)
			if err != nil {
				return errorResult(fmt.Sprintf("list_project_tasks: auth: %v", err)), nil, nil
			}
			plans, err := svcs.Projects.ListMine(ctx, profile)
			if err != nil {
				return errorResult(fmt.Sprintf("list_project_tasks: list plans: %v", err)), nil, nil
			}
			for _, plan := range plans {
				if plan.MyTaskCount == 0 {
					continue
				}
				planTasks, err := svcs.Projects.ListTasks(ctx, profile, plan.ProjectID, plan.ID)
				if err != nil {
					return errorResult(fmt.Sprintf("list_project_tasks: list tasks: %v", err)), nil, nil
				}
				for _, t := range planTasks {
					if t.AssignedTo(user.UID) {
						tasks = append(tasks, t)
					}
				}
			}
		} else {
			var err error
			tasks, err = svcs.Projects.ListTasks(ctx, profile, args.ProjectID, args.PlanID)
			if err != nil {
				return errorResult("list_project_tasks: " + err.Error()), nil, nil
			}
		}

		if len(tasks) > limit {
			tasks = tasks[:limit]
		}

		out := make([]projectTaskRowJSON, 0, len(tasks))
		for _, t := range tasks {
			out = append(out, toProjectTaskRowJSON(t))
		}
		return jsonResult(struct {
			Schema string               `json:"schema"`
			Tasks  []projectTaskRowJSON `json:"tasks"`
		}{Schema: "tdx.v1.projectTaskList", Tasks: out})
	}
}

func getProjectTaskHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getProjectTaskArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getProjectTaskArgs) (*sdkmcp.CallToolResult, any, error) {
		if args.ProjectID <= 0 || args.PlanID <= 0 || args.TaskID <= 0 {
			return errorResult("get_project_task: projectID, planID, and taskID are all required"), nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		task, err := svcs.Projects.GetTask(ctx, profile, args.ProjectID, args.PlanID, args.TaskID)
		if err != nil {
			return errorResult("get_project_task: " + err.Error()), nil, nil
		}
		return jsonResult(struct {
			Schema string                `json:"schema"`
			Task   projectTaskDetailJSON `json:"task"`
		}{Schema: "tdx.v1.projectTask", Task: toProjectTaskDetailJSON(task)})
	}
}

func getProjectFeedHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getProjectFeedArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getProjectFeedArgs) (*sdkmcp.CallToolResult, any, error) {
		if args.ProjectID <= 0 {
			return errorResult("get_project_feed: projectID is required"), nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		entries, err := svcs.Projects.GetFeed(ctx, profile, args.ProjectID)
		if err != nil {
			return errorResult("get_project_feed: " + err.Error()), nil, nil
		}
		if args.Limit > 0 && len(entries) > args.Limit {
			entries = entries[:args.Limit]
		}
		out := make([]projectFeedEntryJSON, 0, len(entries))
		for _, e := range entries {
			ts := ""
			if !e.CreatedDate.IsZero() {
				ts = e.CreatedDate.Format(time.RFC3339)
			}
			out = append(out, projectFeedEntryJSON{
				ID:            e.ID,
				CreatedByUID:  e.CreatedByUID,
				CreatedByName: e.CreatedByName,
				CreatedDate:   ts,
				UpdateType:    e.UpdateTypeLabel(),
				Body:          e.Body,
				IsPrivate:     e.IsPrivate,
				LikesCount:    e.LikesCount,
				RepliesCount:  e.RepliesCount,
			})
		}
		return jsonResult(struct {
			Schema    string                 `json:"schema"`
			ProjectID int                    `json:"projectID"`
			Entries   []projectFeedEntryJSON `json:"entries"`
		}{Schema: "tdx.v1.projectFeed", ProjectID: args.ProjectID, Entries: out})
	}
}

func getProjectTimeReviewHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getProjectTimeReviewArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getProjectTimeReviewArgs) (*sdkmcp.CallToolResult, any, error) {
		if args.ProjectID <= 0 {
			return errorResult("get_project_time_review: projectID is required"), nil, nil
		}
		if args.AllUsers && len(args.UserUIDs) > 0 {
			return errorResult("get_project_time_review: userUIDs and allUsers are mutually exclusive"), nil, nil
		}
		if args.Week != "" && (args.From != "" || args.To != "") {
			return errorResult("get_project_time_review: week and from/to are mutually exclusive"), nil, nil
		}
		if (args.From != "") != (args.To != "") {
			return errorResult("get_project_time_review: from and to must be given together"), nil, nil
		}

		profile := resolveProfile(svcs, args.Profile)

		// Resolve date range.
		var rng domain.DateRange
		switch {
		case args.Week != "":
			d, err := time.ParseInLocation("2006-01-02", args.Week, domain.EasternTZ)
			if err != nil {
				return errorResult(fmt.Sprintf("get_project_time_review: invalid week: %v", err)), nil, nil
			}
			w := domain.WeekRefContaining(d)
			rng = domain.DateRange{From: w.StartDate, To: w.EndDate}
		case args.From != "":
			from, err := time.ParseInLocation("2006-01-02", args.From, domain.EasternTZ)
			if err != nil {
				return errorResult(fmt.Sprintf("get_project_time_review: invalid from: %v", err)), nil, nil
			}
			to, err := time.ParseInLocation("2006-01-02", args.To, domain.EasternTZ)
			if err != nil {
				return errorResult(fmt.Sprintf("get_project_time_review: invalid to: %v", err)), nil, nil
			}
			rng = domain.DateRange{From: from, To: to}
		default:
			w := domain.WeekRefContaining(time.Now())
			rng = domain.DateRange{From: w.StartDate, To: w.EndDate}
		}

		if span := domain.WeekSpan(rng.From, rng.To); span > domain.MaxReportWeeks {
			return errorResult(fmt.Sprintf("%v: weeks=%d max=%d; narrow the from/to range",
				domain.ErrFanoutLimitExceeded, span, domain.MaxReportWeeks)), nil, nil
		}

		// Resolve users.
		var users []domain.User
		switch {
		case args.AllUsers:
			if svcs.Projects == nil {
				return errorResult("get_project_time_review: project service not configured"), nil, nil
			}
			resources, err := svcs.Projects.ListResources(ctx, profile, args.ProjectID)
			if err != nil {
				return errorResult(fmt.Sprintf("get_project_time_review: list resources: %v", err)), nil, nil
			}
			users = make([]domain.User, 0, len(resources))
			for _, r := range resources {
				users = append(users, domain.User{UID: r.UID, FullName: r.FullName})
			}
		case len(args.UserUIDs) > 0:
			users = make([]domain.User, 0, len(args.UserUIDs))
			for _, uid := range args.UserUIDs {
				users = append(users, domain.User{UID: uid})
			}
		default:
			// Default: authenticated user.
			if svcs.Auth == nil {
				return errorResult("get_project_time_review: auth service not configured"), nil, nil
			}
			me, err := svcs.Auth.WhoAmI(ctx, profile)
			if err != nil {
				return errorResult(fmt.Sprintf("get_project_time_review: whoami: %v", err)), nil, nil
			}
			users = []domain.User{{UID: me.UID, FullName: me.FullName}}
		}

		if len(users) > domain.MaxReportUsers {
			return errorResult(fmt.Sprintf("%v: users=%d max=%d; narrow with userUIDs or a smaller team",
				domain.ErrFanoutLimitExceeded, len(users), domain.MaxReportUsers)), nil, nil
		}

		// Fan-out per user.
		mu := sync.Mutex{}
		allEntries := make([]domain.TimeEntry, 0, len(users)*10)

		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(5)
		for _, u := range users {
			u := u
			g.Go(func() error {
				entries, err := svcs.Time.SearchEntries(gctx, profile, domain.EntryFilter{
					DateRange: rng,
					UserUID:   u.UID,
					ProjectID: args.ProjectID,
					PlanID:    args.PlanID,
					TaskID:    args.TaskID,
					Limit:     args.Limit,
				})
				if err != nil {
					return fmt.Errorf("search for %s: %w", u.UID, err)
				}
				mu.Lock()
				allEntries = append(allEntries, entries...)
				mu.Unlock()
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return errorResult(fmt.Sprintf("get_project_time_review: %v", err)), nil, nil
		}

		// Sort by date then UID.
		sort.SliceStable(allEntries, func(i, j int) bool {
			if !allEntries[i].Date.Equal(allEntries[j].Date) {
				return allEntries[i].Date.Before(allEntries[j].Date)
			}
			return allEntries[i].UserUID < allEntries[j].UserUID
		})

		uidToName := make(map[string]string, len(users))
		for _, u := range users {
			uidToName[u.UID] = u.FullName
		}

		totalMin, billMin, nonBillMin := 0, 0, 0
		for _, e := range allEntries {
			totalMin += e.Minutes
			if e.Billable {
				billMin += e.Minutes
			} else {
				nonBillMin += e.Minutes
			}
		}

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

		return jsonResult(struct {
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
			ProjectID: args.ProjectID,
			DateRange: map[string]string{
				"from": rng.From.Format("2006-01-02"),
				"to":   rng.To.Format("2006-01-02"),
			},
			Users:            usersJSON,
			TotalHours:       float64(totalMin) / 60.0,
			BillableHours:    float64(billMin) / 60.0,
			NonBillableHours: float64(nonBillMin) / 60.0,
			Entries:          entriesJSON,
		})
	}
}

func getProjectTaskFeedHandler(svcs Services) func(context.Context, *sdkmcp.CallToolRequest, getProjectTaskFeedArgs) (*sdkmcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *sdkmcp.CallToolRequest, args getProjectTaskFeedArgs) (*sdkmcp.CallToolResult, any, error) {
		if args.ProjectID <= 0 || args.PlanID <= 0 || args.TaskID <= 0 {
			return errorResult("get_project_task_feed: projectID, planID, and taskID are all required"), nil, nil
		}
		profile := resolveProfile(svcs, args.Profile)
		entries, err := svcs.Projects.GetTaskFeed(ctx, profile, args.ProjectID, args.PlanID, args.TaskID)
		if err != nil {
			return errorResult("get_project_task_feed: " + err.Error()), nil, nil
		}
		if args.Limit > 0 && len(entries) > args.Limit {
			entries = entries[:args.Limit]
		}
		out := make([]projectFeedEntryJSON, 0, len(entries))
		for _, e := range entries {
			ts := ""
			if !e.CreatedDate.IsZero() {
				ts = e.CreatedDate.Format(time.RFC3339)
			}
			out = append(out, projectFeedEntryJSON{
				ID:            e.ID,
				CreatedByUID:  e.CreatedByUID,
				CreatedByName: e.CreatedByName,
				CreatedDate:   ts,
				UpdateType:    e.UpdateTypeLabel(),
				Body:          e.Body,
				IsPrivate:     e.IsPrivate,
				LikesCount:    e.LikesCount,
				RepliesCount:  e.RepliesCount,
			})
		}
		return jsonResult(struct {
			Schema    string                 `json:"schema"`
			ProjectID int                    `json:"projectID"`
			PlanID    int                    `json:"planID"`
			TaskID    int                    `json:"taskID"`
			Entries   []projectFeedEntryJSON `json:"entries"`
		}{
			Schema:    "tdx.v1.projectTaskFeed",
			ProjectID: args.ProjectID,
			PlanID:    args.PlanID,
			TaskID:    args.TaskID,
			Entries:   out,
		})
	}
}
