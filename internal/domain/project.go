package domain

import (
	"fmt"
	"strings"
	"time"
)

// ProjectPlanType normalizes TD's PlanType enum.
//
//	1 → Waterfall, 2 → Cardwall, anything else → "unknown(N)"
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
	ID              int       `json:"id"                    yaml:"id"`
	Name            string    `json:"name"                  yaml:"name"`
	StatusID        int       `json:"statusID,omitempty"    yaml:"statusID,omitempty"`
	StatusName      string    `json:"statusName,omitempty"  yaml:"statusName,omitempty"`
	TypeID          int       `json:"typeID,omitempty"      yaml:"typeID,omitempty"`
	TypeName        string    `json:"typeName,omitempty"    yaml:"typeName,omitempty"`
	AccountID       int       `json:"accountID,omitempty"   yaml:"accountID,omitempty"`
	AccountName     string    `json:"accountName,omitempty" yaml:"accountName,omitempty"`
	ManagerUID      string    `json:"managerUID,omitempty"  yaml:"managerUID,omitempty"`  // decoded from TD's AdminUID
	ManagerName     string    `json:"managerName,omitempty" yaml:"managerName,omitempty"` // decoded from TD's AdminName
	SponsorUID      string    `json:"sponsorUID,omitempty"  yaml:"sponsorUID,omitempty"`
	SponsorName     string    `json:"sponsorName,omitempty" yaml:"sponsorName,omitempty"`
	PercentComplete float64   `json:"percentComplete,omitempty" yaml:"percentComplete,omitempty"`
	EstimatedHours  float64   `json:"estimatedHours,omitempty"  yaml:"estimatedHours,omitempty"`
	ActualHours     float64   `json:"actualHours,omitempty"     yaml:"actualHours,omitempty"`
	StartDate       time.Time `json:"startDate,omitempty"   yaml:"startDate,omitempty"`
	EndDate         time.Time `json:"endDate,omitempty"     yaml:"endDate,omitempty"`
	ModifiedDate    time.Time `json:"modifiedDate,omitempty" yaml:"modifiedDate,omitempty"`
	IsActive        bool      `json:"isActive,omitempty"    yaml:"isActive,omitempty"`
	Description     string    `json:"description,omitempty" yaml:"description,omitempty"`
}

// ProjectSearchFilter mirrors POST /api/projects/search body fields tdx surfaces.
// All zero values mean "don't filter".
type ProjectSearchFilter struct {
	NameLike   string
	ManagerUID string
	StatusIDs  []int
	TypeIDs    []int
	IsActive   *bool
	IsOpen     *bool
	MaxResults int
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
	TaskCount       int             `json:"taskCount,omitempty"       yaml:"taskCount,omitempty"`
	MyTaskCount     int             `json:"myTaskCount,omitempty"     yaml:"myTaskCount,omitempty"`
	PercentComplete float64         `json:"percentComplete,omitempty" yaml:"percentComplete,omitempty"`
	EstimatedHours  float64         `json:"estimatedHours,omitempty"  yaml:"estimatedHours,omitempty"`
	ActualHours     float64         `json:"actualHours,omitempty"     yaml:"actualHours,omitempty"`
	StartDate       time.Time       `json:"startDate,omitempty"    yaml:"startDate,omitempty"`
	EndDate         time.Time       `json:"endDate,omitempty"      yaml:"endDate,omitempty"`
	ModifiedDate    time.Time       `json:"modifiedDate,omitempty" yaml:"modifiedDate,omitempty"`
	IsCheckedOut    bool            `json:"isCheckedOut,omitempty" yaml:"isCheckedOut,omitempty"`
}

// ProjectTaskResource is a row in ProjectTask.Resources (assignment).
type ProjectTaskResource struct {
	UID             string  `json:"uid"`
	FullName        string  `json:"fullName"`
	PercentAssigned float64 `json:"percentAssigned,omitempty"`
	RoleID          int     `json:"roleID,omitempty"`
	RoleName        string  `json:"roleName,omitempty"`
}

// ProjectTask is a row in /api/projects/{p}/plans/{pl}/tasks.
type ProjectTask struct {
	ProjectID       int                   `json:"projectID"`
	PlanID          int                   `json:"planID"`
	PlanName        string                `json:"planName,omitempty"`
	ID              int                   `json:"taskID"`
	Title           string                `json:"title"`
	Status          string                `json:"status,omitempty"` // TD returns a string here ("InProcess")
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
	TicketAppID int `json:"ticketAppID,omitempty"`
	TicketID    int `json:"ticketID,omitempty"`
}

// ProjectResource is one row from /api/projects/{id}/resources.
// Minimal Phase 4 shape — only the fields needed for the time-review
// "team" path. Wire UID field is lowercase ("UID", not "UserUID").
type ProjectResource struct {
	UID      string `json:"uid"                yaml:"uid"`
	FullName string `json:"fullName,omitempty" yaml:"fullName,omitempty"`
	RoleID   int    `json:"roleID,omitempty"   yaml:"roleID,omitempty"`
	RoleName string `json:"roleName,omitempty" yaml:"roleName,omitempty"`
	IsActive bool   `json:"isActive,omitempty" yaml:"isActive,omitempty"`
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
