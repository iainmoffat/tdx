# `tdx project` MVP (Phase 1) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to execute this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Ship `tdx project ...` Phase 1 — 7 commands (6 read + 1 time-crossover write) + 7 MCP tools. Mirror the `tdx ticket` package shape.

**Spec:** `docs/specs/2026-05-11-tdx-project-mvp.md`

**Architecture:** Net-new `internal/svc/projectsvc/` + `internal/cli/project/` + `internal/mcp/tools_project.go` packages. Additive domain types (`Project`, `ProjectPlan`, `ProjectTask`, `ProjectPlanType`). Additive `Target.PlanID` field. Reuse existing `timesvc.AddEntry` for `tdx project log`.

**Wire-format facts confirmed by live probe on UFL (2026-05-11):**

- `/api/projects/list` returns **plan-shaped** objects. Each row carries `ID` (= plan ID), `Title`, `ProjectID`, `ProjectName`, `TaskCount`, `MyTaskCount`, `PlanType` (1=waterfall, 2=cardwall), plus a smattering of plan-state fields (`DraftID`, `IsCheckedOut`, `CurrentVersion`).
- `/api/projects/search` returns full project records. NameLike + IsActive honored.
- `/api/projects/{id}` has manager fields as `AdminUID` / `AdminName` / `AdminEmail` (not `Manager*`). Also `SponsorUID`/`SponsorName`/`SponsorEmail` (separate role).
- `/api/projects/{projectID}/plans/{planID}/tasks` returns tasks. **No `Responsible*` field** — assignment lives in `Resources[]`: each entry has `ResourceUID` (uppercase GUID), `ResourceFullName`, `PercentAssignedWhole`, `ResourceRoleID`, `ResourceRoleName`. Status is a string field (`"InProcess"`); `StatusID` is numeric. Date fields are `StartDateUtc` / `EndDateUtc` (no plain `StartDate`/`EndDate`). Hierarchy via `IsParent`/`IndentLevel`/`ParentID`/`OutlineNumber`.

---

## File structure

```
internal/
├── domain/
│   ├── project.go                   # NEW: Project, ProjectPlan, ProjectPlanType, ProjectTask, ProjectTaskResource, ProjectType, ProjectSearchFilter
│   ├── project_test.go              # NEW
│   └── target.go                    # MODIFY: add PlanID int field (+ Validate update for TargetProjectTask)
├── svc/
│   └── projectsvc/                  # NEW package
│       ├── service.go               # Service constructor (mirrors ticketsvc)
│       ├── types.go                 # wireProject, wireProjectSearch, wirePlan, wirePlanSearch, wireTask, wireTaskResource
│       ├── projects.go              # ListMine, Search, Get
│       ├── projects_test.go
│       ├── plans.go                 # SearchPlans
│       ├── plans_test.go
│       ├── tasks.go                 # ListTasks, GetTask
│       ├── tasks_test.go
│       ├── types_lookup.go          # ListProjectTypes, ResolveTypeByName
│       └── types_lookup_test.go
├── cli/
│   └── project/                     # NEW package
│       ├── project.go               # Top-level + projectsvcAPI + New()
│       ├── helpers.go               # peoplesvcAPI (mirror), resolvePrincipal, printProjectList, printPlanList, printTaskList, formatDate
│       ├── helpers_test.go
│       ├── list.go                  # tdx project list
│       ├── list_test.go
│       ├── search.go                # tdx project search
│       ├── search_test.go
│       ├── show.go                  # tdx project show
│       ├── show_test.go
│       ├── plan.go                  # tdx project plan list
│       ├── plan_test.go
│       ├── task.go                  # tdx project task list (single-plan + --mine) + tdx project task show
│       ├── task_test.go
│       ├── log.go                   # tdx project log
│       └── log_test.go
├── mcp/
│   ├── tools_project.go             # NEW: read-only tools
│   ├── tools_project_test.go
│   └── tools_project_mutating.go    # NEW: log_project_task_time
└── cli/cli.go                       # MODIFY: register newProjectCmd()
docs/
├── specs/2026-05-11-tdx-project-mvp.md           # already committed
├── plans/2026-05-11-tdx-project-mvp.md           # this file
└── guide/
    ├── project.md                   # NEW
    └── ../guide.md                  # MODIFY (TOC entry)
README.md                            # MODIFY (ASCII tree gets `project`)
```

---

## Tasks

### Task 1: Domain types

**Files:**
- Create: `internal/domain/project.go`
- Create: `internal/domain/project_test.go`
- Modify: `internal/domain/target.go` (add `PlanID int` field)

- [ ] **Step 1.1: Create `internal/domain/project.go`**

```go
package domain

import "fmt"

// ProjectPlanType normalizes TD's PlanType enum.
//   1 → Waterfall, 2 → Cardwall, anything else → "unknown(N)"
type ProjectPlanType int

const (
    PlanWaterfall ProjectPlanType = 1
    PlanCardwall  ProjectPlanType = 2
)

func (t ProjectPlanType) String() string {
    switch t {
    case PlanWaterfall:
        return "waterfall"
    case PlanCardwall:
        return "cardwall"
    default:
        return fmt.Sprintf("unknown(%d)", int(t))
    }
}

// Project is the canonical project record. Populated fully by GetProject;
// partially by SearchProjects and ListMyProjects (those leave plan-only
// or detail-only fields zero — callers should treat absence as "unknown",
// not "false"/"zero").
type Project struct {
    ID              int       `json:"id"                   yaml:"id"`
    Name            string    `json:"name"                 yaml:"name"`
    StatusID        int       `json:"statusID,omitempty"   yaml:"statusID,omitempty"`
    StatusName      string    `json:"statusName,omitempty" yaml:"statusName,omitempty"`
    TypeID          int       `json:"typeID,omitempty"     yaml:"typeID,omitempty"`
    TypeName        string    `json:"typeName,omitempty"   yaml:"typeName,omitempty"`
    AccountID       int       `json:"accountID,omitempty"  yaml:"accountID,omitempty"`
    AccountName     string    `json:"accountName,omitempty" yaml:"accountName,omitempty"`
    ManagerUID      string    `json:"managerUID,omitempty" yaml:"managerUID,omitempty"`   // decoded from TD's AdminUID
    ManagerName     string    `json:"managerName,omitempty" yaml:"managerName,omitempty"` // decoded from TD's AdminName
    SponsorUID      string    `json:"sponsorUID,omitempty" yaml:"sponsorUID,omitempty"`
    SponsorName     string    `json:"sponsorName,omitempty" yaml:"sponsorName,omitempty"`
    PercentComplete float64   `json:"percentComplete,omitempty" yaml:"percentComplete,omitempty"`
    EstimatedHours  float64   `json:"estimatedHours,omitempty"  yaml:"estimatedHours,omitempty"`
    ActualHours     float64   `json:"actualHours,omitempty"     yaml:"actualHours,omitempty"`
    StartDate       time.Time `json:"startDate,omitempty"  yaml:"startDate,omitempty"`
    EndDate         time.Time `json:"endDate,omitempty"    yaml:"endDate,omitempty"`
    ModifiedDate    time.Time `json:"modifiedDate,omitempty" yaml:"modifiedDate,omitempty"`
    IsActive        bool      `json:"isActive,omitempty"   yaml:"isActive,omitempty"`
    Description     string    `json:"description,omitempty" yaml:"description,omitempty"`
}

// ProjectSearchFilter mirrors POST /api/projects/search body fields tdx surfaces.
// All zero values mean "don't filter".
type ProjectSearchFilter struct {
    NameLike    string
    ManagerUID  string
    StatusIDs   []int
    TypeIDs     []int
    IsActive    *bool
    IsOpen      *bool
    MaxResults  int
}

// ProjectType is a /api/projects/types row (id + name + isActive).
type ProjectType struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    IsActive bool   `json:"isActive,omitempty"`
}

// ProjectPlan is a row in /api/projects/list or /api/projects/{id}/plans/search.
// Note: ProjectID and PlanID are distinct — ID is the plan ID; the encompassing
// project is in ProjectID/ProjectName.
type ProjectPlan struct {
    ID              int             `json:"planID"      yaml:"planID"`
    ProjectID       int             `json:"projectID"   yaml:"projectID"`
    ProjectName     string          `json:"projectName" yaml:"projectName"`
    Title           string          `json:"title"       yaml:"title"`
    Type            ProjectPlanType `json:"type"        yaml:"type"`
    TaskCount       int             `json:"taskCount,omitempty"   yaml:"taskCount,omitempty"`
    MyTaskCount     int             `json:"myTaskCount,omitempty" yaml:"myTaskCount,omitempty"`
    PercentComplete float64         `json:"percentComplete,omitempty" yaml:"percentComplete,omitempty"`
    EstimatedHours  float64         `json:"estimatedHours,omitempty"  yaml:"estimatedHours,omitempty"`
    ActualHours     float64         `json:"actualHours,omitempty"     yaml:"actualHours,omitempty"`
    StartDate       time.Time       `json:"startDate,omitempty"   yaml:"startDate,omitempty"`
    EndDate         time.Time       `json:"endDate,omitempty"     yaml:"endDate,omitempty"`
    ModifiedDate    time.Time       `json:"modifiedDate,omitempty" yaml:"modifiedDate,omitempty"`
    IsCheckedOut    bool            `json:"isCheckedOut,omitempty" yaml:"isCheckedOut,omitempty"`
}

// ProjectTaskResource is a row in ProjectTask.Resources (assignment).
type ProjectTaskResource struct {
    UID               string  `json:"uid"`
    FullName          string  `json:"fullName"`
    PercentAssigned   float64 `json:"percentAssigned,omitempty"`
    RoleID            int     `json:"roleID,omitempty"`
    RoleName          string  `json:"roleName,omitempty"`
}

// ProjectTask is a row in /api/projects/{p}/plans/{pl}/tasks.
type ProjectTask struct {
    ProjectID       int                   `json:"projectID"`
    PlanID          int                   `json:"planID"`
    PlanName        string                `json:"planName,omitempty"`
    ID              int                   `json:"taskID"`
    Title           string                `json:"title"`
    Status          string                `json:"status,omitempty"`   // TD returns a string here ("InProcess")
    StatusID        int                   `json:"statusID,omitempty"`
    PercentComplete float64               `json:"percentComplete,omitempty"`
    EstimatedHours  float64               `json:"estimatedHours,omitempty"`
    ActualHours     float64               `json:"actualHours,omitempty"`
    RemainingHours  float64               `json:"remainingHours,omitempty"`
    StartDate       time.Time             `json:"startDate,omitempty"`
    EndDate         time.Time             `json:"endDate,omitempty"`
    ModifiedDate    time.Time             `json:"modifiedDate,omitempty"`
    IsParent        bool                  `json:"isParent,omitempty"`
    IndentLevel     int                   `json:"indentLevel,omitempty"`
    ParentID        int                   `json:"parentID,omitempty"`
    OutlineNumber   string                `json:"outlineNumber,omitempty"`
    Description     string                `json:"description,omitempty"`
    Resources       []ProjectTaskResource `json:"resources,omitempty"`
    // Crossover signals — useful in Phase 2:
    TicketAppID     int                   `json:"ticketAppID,omitempty"`
    TicketID        int                   `json:"ticketID,omitempty"`
}

// AssignedTo reports whether the given UID is among the task's resources
// (case-insensitive — TD returns task resource UIDs in UPPERCASE but
// the User UID is lowercase).
func (t ProjectTask) AssignedTo(uid string) bool {
    if uid == "" {
        return false
    }
    for _, r := range t.Resources {
        if strings.EqualFold(r.UID, uid) {
            return true
        }
    }
    return false
}
```

Add `time` and `strings` to imports.

- [ ] **Step 1.2: Write `internal/domain/project_test.go`**

```go
package domain

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestProjectPlanType_String(t *testing.T) {
    require.Equal(t, "waterfall", PlanWaterfall.String())
    require.Equal(t, "cardwall", PlanCardwall.String())
    require.Equal(t, "unknown(0)", ProjectPlanType(0).String())
    require.Equal(t, "unknown(99)", ProjectPlanType(99).String())
}

func TestProjectTask_AssignedTo_CaseInsensitive(t *testing.T) {
    me := "61fc4d29-1a09-ef11-86d4-df13b8e4e655"
    task := ProjectTask{
        Resources: []ProjectTaskResource{
            {UID: "ABCD1234-AAAA-BBBB-CCCC-DDDDEEEEFFFF", FullName: "Someone"},
            {UID: strings.ToUpper(me), FullName: "Me"},
        },
    }
    require.True(t, task.AssignedTo(me))
    require.True(t, task.AssignedTo(strings.ToUpper(me)))
    require.False(t, task.AssignedTo("nobody"))
    require.False(t, task.AssignedTo(""))
}

func TestProjectTask_AssignedTo_EmptyResources(t *testing.T) {
    task := ProjectTask{}
    require.False(t, task.AssignedTo("anything"))
}
```

- [ ] **Step 1.3: Modify `internal/domain/target.go` to add PlanID**

Find the `Target` struct and add `PlanID int` between `TaskID` and `ProjectID`:

```go
type Target struct {
    Kind        TargetKind `json:"kind" yaml:"kind"`
    AppID       int        `json:"appID" yaml:"appID"`
    ItemID      int        `json:"itemID" yaml:"itemID"`
    TaskID      int        `json:"taskID,omitempty" yaml:"taskID,omitempty"`
    PlanID      int        `json:"planID,omitempty" yaml:"planID,omitempty"`   // NEW: project-task plan ID
    ProjectID   int        `json:"projectID,omitempty" yaml:"projectID,omitempty"`
    DisplayName string     `json:"displayName,omitempty" yaml:"displayName,omitempty"`
    DisplayRef  string     `json:"displayRef,omitempty" yaml:"displayRef,omitempty"`
    GroupName   string     `json:"groupName,omitempty" yaml:"groupName,omitempty"`
}
```

No `Validate` change required — `PlanID` is metadata for wire encoding; absence is acceptable (the timesvc encode helper already pulls PlanID from the wire path indirectly through other means; we surface it explicitly so test fixtures are clearer).

- [ ] **Step 1.4: Verify**

```bash
go test ./internal/domain/... -v 2>&1 | tail -20
go vet ./...
```

- [ ] **Step 1.5: Commit**

```
feat(domain): add Project, ProjectPlan, ProjectTask types; Target.PlanID

Adds the domain layer for the v0.17.0 tdx project MVP. Plan-type enum
normalized (waterfall=1, cardwall=2). ProjectTask.AssignedTo does a
case-insensitive UID match (TD returns task resource UIDs uppercase).
```

---

### Task 2: Service layer — projectsvc

**Files:**
- Create: `internal/svc/projectsvc/service.go`
- Create: `internal/svc/projectsvc/types.go`
- Create: `internal/svc/projectsvc/projects.go` + `_test.go`
- Create: `internal/svc/projectsvc/plans.go` + `_test.go`
- Create: `internal/svc/projectsvc/tasks.go` + `_test.go`
- Create: `internal/svc/projectsvc/types_lookup.go` + `_test.go`

- [ ] **Step 2.1: `service.go`** — copy the `ticketsvc/service.go` shape verbatim, minus `resolveAppID`. Constructor: `New(paths config.Paths) *Service`. `clientFor(profileName)` builds the authed `tdx.Client`.

- [ ] **Step 2.2: `types.go`** — wire structs.

```go
type wireProject struct {
    ID              int     `json:"ID"`
    Name            string  `json:"Name"`
    StatusID        int     `json:"StatusID,omitempty"`
    StatusName      string  `json:"StatusName,omitempty"`
    TypeID          int     `json:"TypeID,omitempty"`
    TypeName        string  `json:"TypeName,omitempty"`
    AccountID       int     `json:"AccountID,omitempty"`
    AccountName     string  `json:"AccountName,omitempty"`
    AdminUID        string  `json:"AdminUID,omitempty"`
    AdminName       string  `json:"AdminName,omitempty"`
    SponsorUID      string  `json:"SponsorUID,omitempty"`
    SponsorName     string  `json:"SponsorName,omitempty"`
    PercentComplete float64 `json:"PercentComplete,omitempty"`
    EstimatedHours  float64 `json:"EstimatedHours,omitempty"`
    ActualHours     float64 `json:"ActualHours,omitempty"`
    StartDate       string  `json:"StartDate,omitempty"`
    EndDate         string  `json:"EndDate,omitempty"`
    ModifiedDate    string  `json:"ModifiedDate,omitempty"`
    IsActive        bool    `json:"IsActive,omitempty"`
    Description     string  `json:"Description,omitempty"`
}

type wireProjectSearch struct {
    NameLike   string `json:"NameLike,omitempty"`
    ManagerUID string `json:"ManagerUID,omitempty"`
    StatusIDs  []int  `json:"StatusIDs,omitempty"`
    TypeIDs    []int  `json:"TypeIDs,omitempty"`
    IsActive   *bool  `json:"IsActive,omitempty"`
    IsOpen     *bool  `json:"IsOpen,omitempty"`
    MaxResults int    `json:"MaxResults,omitempty"`
}

type wirePlan struct {
    ID              int     `json:"ID"`        // plan ID
    Title           string  `json:"Title"`
    ProjectID       int     `json:"ProjectID"`
    ProjectName     string  `json:"ProjectName"`
    TaskCount       int     `json:"TaskCount,omitempty"`
    MyTaskCount     int     `json:"MyTaskCount,omitempty"`
    PlanType        int     `json:"PlanType,omitempty"`
    PercentComplete float64 `json:"PercentComplete,omitempty"`
    EstimatedHours  float64 `json:"EstimatedHours,omitempty"`
    ActualHours     float64 `json:"ActualHours,omitempty"`
    StartDateUtc    string  `json:"StartDateUtc,omitempty"`
    EndDateUtc      string  `json:"EndDateUtc,omitempty"`
    ModifiedDate    string  `json:"ModifiedDate,omitempty"`
    IsCheckedOut    bool    `json:"IsCheckedOut,omitempty"`
}

type wirePlanSearch struct {
    NameLike     string `json:"NameLike,omitempty"`
    IncludeEmpty bool   `json:"IncludeEmpty,omitempty"`
}

type wireTaskResource struct {
    ResourceUID          string  `json:"ResourceUID"`
    ResourceFullName     string  `json:"ResourceFullName,omitempty"`
    PercentAssignedWhole float64 `json:"PercentAssignedWhole,omitempty"`
    ResourceRoleID       int     `json:"ResourceRoleID,omitempty"`
    ResourceRoleName     string  `json:"ResourceRoleName,omitempty"`
}

type wireTask struct {
    ID              int                `json:"ID"`
    Title           string             `json:"Title"`
    ProjectID       int                `json:"ProjectID"`
    ProjectName     string             `json:"ProjectName,omitempty"`
    PlanID          int                `json:"PlanID"`
    PlanName        string             `json:"PlanName,omitempty"`
    Status          string             `json:"Status,omitempty"`
    StatusID        int                `json:"StatusID,omitempty"`
    PercentComplete float64            `json:"PercentComplete,omitempty"`
    EstimatedHours  float64            `json:"EstimatedHours,omitempty"`
    ActualHours     float64            `json:"ActualHours,omitempty"`
    RemainingHours  float64            `json:"RemainingHours,omitempty"`
    StartDateUtc    string             `json:"StartDateUtc,omitempty"`
    EndDateUtc      string             `json:"EndDateUtc,omitempty"`
    ModifiedDate    string             `json:"ModifiedDate,omitempty"`
    IsParent        bool               `json:"IsParent,omitempty"`
    IndentLevel     int                `json:"IndentLevel,omitempty"`
    ParentID        int                `json:"ParentID,omitempty"`
    OutlineNumber   string             `json:"OutlineNumber,omitempty"`
    Description     string             `json:"Description,omitempty"`
    Resources       []wireTaskResource `json:"Resources,omitempty"`
    TicketAppID     int                `json:"TicketAppID,omitempty"`
    TicketID        int                `json:"TicketID,omitempty"`
}

type wireProjectType struct {
    ID       int    `json:"ID"`
    Name     string `json:"Name"`
    IsActive bool   `json:"IsActive,omitempty"`
}

// parseTD parses TD's ISO-ish timestamp; returns zero time on empty/sentinel.
func parseTD(s string) time.Time {
    if s == "" || strings.HasPrefix(s, "0001-01-01") || strings.HasPrefix(s, "1900-01-01") {
        return time.Time{}
    }
    for _, layout := range []string{
        time.RFC3339,
        time.RFC3339Nano,
        "2006-01-02T15:04:05",       // TD often omits the zone on plan/task dates
        "2006-01-02T15:04:05.999",
    } {
        if t, err := time.Parse(layout, s); err == nil {
            return t
        }
    }
    return time.Time{}
}
```

- [ ] **Step 2.3: `projects.go` — `ListMine`, `Search`, `Get` (TDD: write tests first via httptest, then implement)**

```go
// ListMine returns plans-shaped rows for projects the caller participates in.
// Note: despite the endpoint name, /api/projects/list returns plan objects
// (one row per plan). Use this for "what am I on"; use SearchProjects for
// project-level search.
func (s *Service) ListMine(ctx context.Context, profile string) ([]domain.ProjectPlan, error) {
    c, err := s.clientFor(profile)
    if err != nil { return nil, err }
    var rows []wirePlan
    if err := c.Get(ctx, "/TDWebApi/api/projects/list", &rows); err != nil {
        return nil, err
    }
    out := make([]domain.ProjectPlan, 0, len(rows))
    for _, w := range rows { out = append(out, decodePlan(w)) }
    return out, nil
}

func (s *Service) Search(ctx context.Context, profile string, f domain.ProjectSearchFilter) ([]domain.Project, error) {
    c, err := s.clientFor(profile)
    if err != nil { return nil, err }
    body := wireProjectSearch{
        NameLike:   f.NameLike,
        ManagerUID: f.ManagerUID,
        StatusIDs:  f.StatusIDs,
        TypeIDs:    f.TypeIDs,
        IsActive:   f.IsActive,
        IsOpen:     f.IsOpen,
        MaxResults: f.MaxResults,
    }
    var rows []wireProject
    if err := c.Post(ctx, "/TDWebApi/api/projects/search", body, &rows); err != nil {
        return nil, err
    }
    out := make([]domain.Project, 0, len(rows))
    for _, w := range rows { out = append(out, decodeProject(w)) }
    return out, nil
}

func (s *Service) Get(ctx context.Context, profile string, id int) (domain.Project, error) {
    c, err := s.clientFor(profile)
    if err != nil { return domain.Project{}, err }
    var w wireProject
    if err := c.Get(ctx, fmt.Sprintf("/TDWebApi/api/projects/%d", id), &w); err != nil {
        return domain.Project{}, err
    }
    return decodeProject(w), nil
}

func decodeProject(w wireProject) domain.Project { /* fill from wire */ }
func decodePlan(w wirePlan) domain.ProjectPlan {
    return domain.ProjectPlan{
        ID:              w.ID,
        ProjectID:       w.ProjectID,
        ProjectName:     w.ProjectName,
        Title:           w.Title,
        Type:            domain.ProjectPlanType(w.PlanType),
        TaskCount:       w.TaskCount,
        MyTaskCount:     w.MyTaskCount,
        PercentComplete: w.PercentComplete,
        EstimatedHours:  w.EstimatedHours,
        ActualHours:     w.ActualHours,
        StartDate:       parseTD(w.StartDateUtc),
        EndDate:         parseTD(w.EndDateUtc),
        ModifiedDate:    parseTD(w.ModifiedDate),
        IsCheckedOut:    w.IsCheckedOut,
    }
}
```

Tests (`projects_test.go`):
- `TestListMine_DecodesPlanRows` — httptest fixture replies with a plan-shaped JSON array; verify `ProjectID`, `ProjectName`, `Title`, `MyTaskCount`, `Type` (=waterfall when PlanType=1).
- `TestSearch_PostsExpectedBody` — fixture captures request body; verify NameLike/IsActive serialize correctly.
- `TestGet_DecodesProjectWithAdminAsManager` — fixture has `AdminUID`/`AdminName`; verify they land in `Project.ManagerUID`/`ManagerName`.
- `TestGet_404Returns_ErrNotFound` (optional, only if `tdx.Client` already surfaces 404; mirror what `ticketsvc/tickets_test.go` does).

- [ ] **Step 2.4: `plans.go` + tests** — `SearchPlans(ctx, profile, projectID, filter)` posts to `/api/projects/{projectID}/plans/search`. Tests cover the basic decode (same shape as ListMine rows).

- [ ] **Step 2.5: `tasks.go` + tests** — `ListTasks(ctx, profile, projectID, planID)` and `GetTask(ctx, profile, projectID, planID, taskID)`. `decodeTask` populates `ProjectID`/`PlanID` from the args if missing on the wire (defensive); populates `Resources` from `wireTaskResource`; parses `StartDateUtc`/`EndDateUtc`. Tests: list decode, task assignment populated.

- [ ] **Step 2.6: `types_lookup.go` + tests** — `ListProjectTypes(ctx, profile, includeInactive bool)` GETs `/api/projects/types?isActive=...`. `ResolveTypeByName(ctx, profile, name)` does case-insensitive exact match; 0 → "not found among N", >1 → "ambiguous (N matches)". Mirror `ticketsvc.ResolveStatusByName` exactly.

- [ ] **Step 2.7: Verify + commit**

```bash
go test ./internal/svc/projectsvc/... -v
```

Commit message:
```
feat(projectsvc): add Service for /api/projects, plans, tasks, types

ListMine / Search / Get + SearchPlans + ListTasks/GetTask +
ListProjectTypes/ResolveTypeByName. Decodes TD's plan-shaped
response from /api/projects/list correctly; maps Admin* fields
to Manager* in the domain layer.
```

---

### Task 3: CLI scaffolding — `internal/cli/project/project.go` + helpers

**Files:**
- Create: `internal/cli/project/project.go`
- Create: `internal/cli/project/helpers.go`
- Create: `internal/cli/project/helpers_test.go`

- [ ] **Step 3.1: `project.go`** — top-level command. Mirror `ticket.go`:

```go
package project

type projectsvcAPI interface {
    ListMine(ctx context.Context, profile string) ([]domain.ProjectPlan, error)
    Search(ctx context.Context, profile string, filter domain.ProjectSearchFilter) ([]domain.Project, error)
    Get(ctx context.Context, profile string, id int) (domain.Project, error)
    SearchPlans(ctx context.Context, profile string, projectID int, nameLike string, includeEmpty bool) ([]domain.ProjectPlan, error)
    ListTasks(ctx context.Context, profile string, projectID, planID int) ([]domain.ProjectTask, error)
    GetTask(ctx context.Context, profile string, projectID, planID, taskID int) (domain.ProjectTask, error)
    ListProjectTypes(ctx context.Context, profile string, includeInactive bool) ([]domain.ProjectType, error)
    ResolveTypeByName(ctx context.Context, profile string, name string) (domain.ProjectType, error)
}

func New() *cobra.Command {
    cmd := &cobra.Command{Use: "project", Short: "Inspect TeamDynamix projects, plans, and tasks"}
    cmd.AddCommand(newListCmd(nil))
    cmd.AddCommand(newSearchCmd(nil))
    cmd.AddCommand(newShowCmd(nil))
    cmd.AddCommand(newPlanCmd(nil))
    cmd.AddCommand(newTaskCmd(nil))
    cmd.AddCommand(newLogCmd(nil))
    return cmd
}
```

- [ ] **Step 3.2: `helpers.go`** — print helpers:
  - `formatDate(t time.Time) string` (copy from ticket)
  - `truncate(s string, n int) string`
  - `printProjectList(w, projects, jsonOut, schema)` — `tdx.v1.projectList` envelope; columns ID, NAME, STATUS, MANAGER, TYPE, % COMPLETE, START, END
  - `printPlanList(w, plans, jsonOut)` — `tdx.v1.projectPlanList`; columns PROJECT-ID, PROJECT, PLAN-ID, PLAN, TYPE, MY-TASKS, TASKS, % COMPLETE, START, END
  - `printTaskList(w, tasks, jsonOut)` — `tdx.v1.projectTaskList`; columns PROJECT, PLAN, TASK-ID, TITLE, STATUS, %, EST, ACT, ASSIGNEES, END
  - `peoplesvcAPI` interface mirroring `ticket/helpers.go` so `--manager me` resolution works
  - `resolvePrincipal(ctx, people, profile, authedUID, arg)` — copy of ticket's
  - `authedUIDFor(ctx, auth, profile)` — copy of ticket's

- [ ] **Step 3.3: Wire `tdx project` into root in `internal/cli/cli.go`**

Find where `tdx ticket` is registered and add a sibling line for `tdx project`. Same place, same shape.

- [ ] **Step 3.4: Verify + commit**

```bash
go build ./...
go vet ./...
```

```
feat(cli/project): scaffold tdx project command tree

Empty subcommand stubs + helpers (printProjectList/printPlanList/
printTaskList). Wires `tdx project` into the root.
```

---

### Task 4: `tdx project list`

**Files:**
- Create: `internal/cli/project/list.go`
- Create: `internal/cli/project/list_test.go`

- [ ] **Step 4.1: TDD — write `list_test.go` first**

Stub `projectsvcAPI`. Tests:
- `TestList_DefaultRendersTable` — stub returns 2 plans; verify table has PROJECT-ID, PROJECT, PLAN, MY-TASKS columns and the project ID/name from the stub.
- `TestList_JSONEnvelope` — verify `--json` emits `schema: "tdx.v1.projectList"` (we use the same envelope for the my-list because the rows ARE plans of projects) — actually, since the rows are plan-shaped, the schema should be `tdx.v1.projectPlanList`. Use the latter to keep schema honesty.

Actually let me revisit: the spec said `tdx.v1.projectList` for `tdx project list`. Given the wire reality (rows are plans), let me use **`tdx.v1.projectPlanList`** and document it as "your projects (one row per plan)". This is more truthful.

- [ ] **Step 4.2: Implement `list.go`** — calls `svc.ListMine`, renders via `printPlanList`.

```
tdx project list [--json] [--limit N] [--profile P]
```

Default limit 50, max 500. Sort: by `ProjectName` then `Title`.

- [ ] **Step 4.3: Verify + commit**

```bash
go test ./internal/cli/project/... -run List -v
```

```
feat(cli/project): tdx project list (your participating plans/projects)
```

---

### Task 5: `tdx project search`

**Files:** `internal/cli/project/search.go` + `_test.go`

- [ ] **Step 5.1: TDD**
- [ ] **Step 5.2: Implement**

```
tdx project search [QUERY] [--manager me|UID|email] [--status NAME|ID]... [--type NAME|ID]... [--active|--include-inactive] [--limit N] [--json] [--profile P]
```

`QUERY` → `NameLike`. `--manager me` resolved via `auth.WhoAmI`. `--status` and `--type` accept numeric (used as-is) or name (resolved via `peoplesvc`-style lookup if available, otherwise number-only acceptable for MVP — `tdx project status list` and similar are out of scope here). `--active` default true; `--include-inactive` flips to false.

Filter fidelity: live-probe will verify which fields TD honors. For MVP, if `ManagerUID` is silently ignored by `/projects/search`, fall back to client-side filter (we already have the project list from search, just match `AdminUID == managerUID`).

Output: `printProjectList` (table or `tdx.v1.projectList` JSON).

- [ ] **Step 5.3: Commit**

```
feat(cli/project): tdx project search (name + filters)
```

---

### Task 6: `tdx project show`

**Files:** `internal/cli/project/show.go` + `_test.go`

- [ ] **Step 6.1: TDD**
- [ ] **Step 6.2: Implement**

```
tdx project show <id> [--json] [--profile P]
```

Renders header:
```
PROJECT 259 — Fiscal Year 2026 Disaster Recovery
Status:        Executing
Type:          Regulatory Project (Regulatory Project, category=UF - IT Project)
Manager:       Charlotte Looney
Sponsor:       Elias Eldayrie
Account:       14300000 (IT-ICT INFRA COMM TECHNOLOGY)
Active:        yes
% Complete:    96.0%
Hours:         actual=58.0 / estimated=320.0
Dates:         2025-07-01 → 2026-06-30
Modified:      2026-05-07
```

JSON envelope `tdx.v1.project`.

Defer "this week" time crossover to a tiny follow-up step within this task IF it's < 30 lines; otherwise omit and note in spec as "Phase 1.1". The crossover requires reading the current week draft (`draftsvc.LoadDraft`) and filtering rows where `Target.ItemID == projectID`. Same pattern as `ticket/show.go`. **Decision:** include it — it's the workflow's payoff.

- [ ] **Step 6.3: Commit**

```
feat(cli/project): tdx project show with this-week time crossover
```

---

### Task 7: `tdx project plan list`

**Files:** `internal/cli/project/plan.go` + `_test.go`

- [ ] **Step 7.1: TDD + implement**

```
tdx project plan list <project-id> [--name-like SUBSTR] [--include-empty] [--json] [--profile P]
```

Calls `svc.SearchPlans`. Renders via `printPlanList` (same columns as `tdx project list`, scoped to one project).

- [ ] **Step 7.2: Commit**

```
feat(cli/project): tdx project plan list
```

---

### Task 8: `tdx project task list` (single-plan + --mine)

**Files:** `internal/cli/project/task.go` + `_test.go`

- [ ] **Step 8.1: TDD**

Tests:
- `TestTaskList_SinglePlan_RendersTable`
- `TestTaskList_Mine_FansOutAcrossProjects` — stub returns 3 plans (one with MyTaskCount=0, two with MyTaskCount>0); for each non-zero plan, stub returns mix of tasks; assert only my-assigned tasks bubble up and `--mine` skips MyTaskCount=0 plans (no API call).
- `TestTaskList_Mine_UpperCaseUIDMatch` — stub returns task with resource UID in uppercase; lowercase authedUID; assert task surfaces.
- `TestTaskList_Mine_RespectsLimit`
- `TestTaskList_RejectsMixedFlags` — `--mine` + `<project-id>` errors.
- `TestTaskList_RequiresPlanWhenProjectGiven` — `tdx project task list 259` without `--plan` errors with a helpful hint.

- [ ] **Step 8.2: Implement single-plan mode**

```
tdx project task list <project-id> --plan <plan-id> [--limit N] [--json] [--profile P]
```

- [ ] **Step 8.3: Implement `--mine` fanout**

```
tdx project task list --mine [--limit N] [--json] [--profile P]
```

Algorithm:
1. `svc.ListMine` → plans
2. Filter to plans with `MyTaskCount > 0`
3. Cap at 50 plans (configurable later)
4. For each plan, `svc.ListTasks(projectID, planID)`, in parallel (errgroup, limit 5)
5. Filter each task list with `task.AssignedTo(authedUID)`
6. Aggregate, sort: `EndDate ASC NULLS LAST, ModifiedDate DESC`
7. Cap at `--limit` (default 50, max 200)
8. Render via `printTaskList`

Helpful empty-state message:
- Zero plans → `no projects/plans assigned to you on this tenant`
- Plans but 0 my-tasks after fanout → `no tasks assigned to you across <N> plans` (note: `Plan.MyTaskCount` was non-zero but tasks themselves don't match — could happen if MyTaskCount counts roles broader than assignment)

- [ ] **Step 8.4: Commit (one commit for the whole task list command)**

```
feat(cli/project): tdx project task list (single-plan + --mine fanout)
```

---

### Task 9: `tdx project task show`

**Files:** task.go (extend) + task_test.go (extend)

- [ ] **Step 9.1: TDD + implement**

```
tdx project task show <project-id> <task-id> --plan <plan-id> [--json] [--profile P]
```

Renders header + Resources + Hours summary + (truncated) Description. JSON envelope `tdx.v1.projectTask`.

- [ ] **Step 9.2: Commit**

```
feat(cli/project): tdx project task show
```

---

### Task 10: `tdx project log` (time crossover)

**Files:** `internal/cli/project/log.go` + `_test.go`

- [ ] **Step 10.1: TDD**

Stub a `timesvcAPI` (same interface as `ticket/log.go`). Tests:
- `TestLog_RequiresYes` — without `--yes`, errors
- `TestLog_BuildsTargetProjectTask` — verify the constructed `Target` has Kind=TargetProjectTask, ItemID=taskID, ProjectID=projectID, PlanID=planID
- `TestLog_HoursMinutesMutex`
- `TestLog_TypeMutex`
- `TestLog_HappyPath` — stub `timesvc.AddEntry` returns a `TimeEntry{ID: 999}`; assert success line includes the entry ID

- [ ] **Step 10.2: Implement**

```
tdx project log <project-id> <task-id> --plan <plan-id>
  --hours N | --minutes N
  --type NAME | --type-id N
  [--date YYYY-MM-DD] [--description "..."] [--billable]
  --yes
  [--profile P]
```

Closely mirror `ticket/log.go`. The only difference is the `Target` construction:

```go
target := domain.Target{
    Kind:      domain.TargetProjectTask,
    ItemID:    taskID,
    PlanID:    planID,
    ProjectID: projectID,
}
```

Validate type-name → ID via `timesvc.TimeTypesForTarget(ctx, profile, target)`.

- [ ] **Step 10.3: Commit**

```
feat(cli/project): tdx project log (log time against a project task)
```

---

### Task 11: MCP tools

**Files:**
- Create: `internal/mcp/tools_project.go`
- Create: `internal/mcp/tools_project_test.go`
- Create: `internal/mcp/tools_project_mutating.go`

- [ ] **Step 11.1: Register read-only tools**

In `tools_project.go`, define `RegisterProjectTools(srv, svcs)` registering 6 tools. Pattern: each tool has an args struct, a handler that resolves the profile and calls `projectsvc`, returns a JSON envelope matching the CLI's. Mirror `tools_ticket.go` exactly.

| Tool | Args |
|---|---|
| `list_my_projects` | `profile? string`, `limit? int` |
| `search_projects` | `profile? string`, `query? string`, `managerUID? string`, `statusIDs? []int`, `typeIDs? []int`, `isActive? *bool`, `isOpen? *bool`, `limit? int` |
| `get_project` | `profile? string`, `id int` |
| `list_project_plans` | `profile? string`, `projectID int`, `nameLike? string`, `includeEmpty? bool` |
| `list_project_tasks` | `profile? string`, `projectID? int`, `planID? int`, `mine? bool`, `limit? int` (validate: `mine` xor (`projectID`+`planID`)) |
| `get_project_task` | `profile? string`, `projectID int`, `planID int`, `taskID int` |

- [ ] **Step 11.2: Register mutating tool**

In `tools_project_mutating.go`, register `log_project_task_time` with `confirm: true` gate (mirror `log_ticket_task_time`).

- [ ] **Step 11.3: Wire registration in `internal/mcp/server.go`**

Find where `RegisterTicketTools(srv, svcs)` is called and add `RegisterProjectTools(srv, svcs)` next to it. Same for `RegisterProjectMutatingTools`.

- [ ] **Step 11.4: Update tool-count documentation** if any (look for `60 → 63` style notes in README/docs).

- [ ] **Step 11.5: Test + commit**

```
feat(mcp): add 7 project tools (6 read + log_project_task_time)
```

---

### Task 12: Live verification on UFL

- [ ] **Step 12.1: Build and run each command against UFL**

```bash
go build -o /tmp/tdx-project ./cmd/tdx

/tmp/tdx-project project list
/tmp/tdx-project project list --json | jq '.[0]'
/tmp/tdx-project project search "Disaster"
/tmp/tdx-project project search --manager me --json | jq 'length'
/tmp/tdx-project project show 259
/tmp/tdx-project project show 259 --json | jq '.project | {id, name, managerName, percentComplete}'
/tmp/tdx-project project plan list 259
/tmp/tdx-project project task list 259 --plan 1292
/tmp/tdx-project project task list --mine
/tmp/tdx-project project task show 259 4938 --plan 1292
```

Spot-verify against TD's web UI. **Filter probes** (record any silently-ignored fields):
- `tdx project search --manager me` → expect projects where AdminUID==my-UID. If we get unrelated projects, `ManagerUID` is silently ignored — fall back to client-side filter and record in `reference_td_search_silent_filters.md`.
- `tdx project search --active=false` → projects with `IsActive==false`. Record if ignored.

- [ ] **Step 12.2: `tdx project log` smoke test**

Pick a known task you're on. Log 0.25h with description "tdx live test". Verify it appears in `tdx time entry list --json` with the right Target. Roll back via `tdx time entry delete <id> --yes`.

- [ ] **Step 12.3: Document any surprises in the spec's post-implementation note section**

- [ ] **Step 12.4: Commit a "live-verify findings" change to the spec if needed**

---

### Task 13: Docs

**Files:**
- Create: `docs/guide/project.md`
- Modify: `docs/guide.md` (TOC + index entries)
- Modify: `README.md` (ASCII command tree)

- [ ] **Step 13.1: Write `docs/guide/project.md`** with sections for each command, examples taken from the live verify. Mirror `docs/guide/time.md` style.

- [ ] **Step 13.2: Modify `docs/guide.md`** to add a "Projects" section pointing at the new page.

- [ ] **Step 13.3: Modify `README.md`** to add `project` to the ASCII command tree alongside `ticket`/`time`/`people`.

- [ ] **Step 13.4: Commit**

```
docs: tdx project guide + index + README tree entry
```

---

### Task 14: PR + tag

- [ ] **Step 14.1: Final lint + test sweep**

```bash
go test ./... && go vet ./... && gofmt -l . && golangci-lint run ./...
```

- [ ] **Step 14.2: Push + open PR**

PR title: `v0.17.0: tdx project MVP (Phase 1)`

- [ ] **Step 14.3: Merge with `--admin` squash, delete branch**

- [ ] **Step 14.4: Tag `v0.17.0` and push** → Goreleaser

- [ ] **Step 14.5: Update memory** — append v0.17.0 release block to `project_tdx_current_state.md`; mark project MVP shipped in `project_tdx_backlog.md`; add a reference memory if any wire-format surprises came up during live probe.

---

## Coverage check vs. spec

- ✅ `tdx project list` → Task 4
- ✅ `tdx project search` → Task 5
- ✅ `tdx project show` (+ this-week crossover) → Task 6
- ✅ `tdx project plan list` → Task 7
- ✅ `tdx project task list` (single-plan + `--mine`) → Task 8
- ✅ `tdx project task show` → Task 9
- ✅ `tdx project log` → Task 10
- ✅ 7 MCP tools → Task 11
- ✅ Domain types + `Target.PlanID` → Task 1
- ✅ Service layer → Task 2
- ✅ Live verify on UFL → Task 12
- ✅ Docs + README → Task 13
- ✅ Release → Task 14
