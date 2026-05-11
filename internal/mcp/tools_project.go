package mcp

import (
	"context"
	"fmt"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iainmoffat/tdx/internal/domain"
)

// --- Argument types ---

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

// RegisterProjectTools registers the 6 read-only project MCP tools.
func RegisterProjectTools(srv *sdkmcp.Server, svcs Services) {
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
