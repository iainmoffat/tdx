package projectsvc

import (
	"strings"
	"time"
)

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
	ID              int     `json:"ID"` // plan ID
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
	OutlineNumber   int                `json:"OutlineNumber,omitempty"` // sequential row order; dotted string is in Wbs
	Wbs             string             `json:"Wbs,omitempty"`
	Description     string             `json:"Description,omitempty"`
	Resources       []wireTaskResource `json:"Resources,omitempty"`
	TicketAppID     int                `json:"TicketAppID,omitempty"`
	TicketID        int                `json:"TicketID,omitempty"`
}

type wireProjectResource struct {
	UID      string `json:"UID"`
	FullName string `json:"FullName,omitempty"`
	RoleID   int    `json:"RoleID,omitempty"`
	RoleName string `json:"RoleName,omitempty"`
	IsActive bool   `json:"IsActive,omitempty"`
}

type wireProjectType struct {
	ID       int    `json:"ID"`
	Name     string `json:"Name"`
	IsActive bool   `json:"IsActive,omitempty"`
}

type wireFeedEntry struct {
	ID              int    `json:"ID"`
	Body            string `json:"Body"`
	CreatedUid      string `json:"CreatedUid,omitempty"`
	CreatedFullName string `json:"CreatedFullName,omitempty"`
	CreatedDate     string `json:"CreatedDate,omitempty"`
	LastUpdatedDate string `json:"LastUpdatedDate,omitempty"`
	UpdateType      int    `json:"UpdateType,omitempty"`
	IsPrivate       bool   `json:"IsPrivate,omitempty"`
	LikesCount      int    `json:"LikesCount,omitempty"`
	RepliesCount    int    `json:"RepliesCount,omitempty"`
}

// wireProjectFeedAdd is the POST body for /api/projects/{id}/feed.
// TD's project-level feed POST uses "Body" for the comment text, not
// "Comments" as on the ticket/task feed endpoints. Confirmed by live
// probe 2026-05-12: a POST with "Comments" returns ID:-1 and silently
// no-ops; the same body with "Body" returns a real entry ID.
type wireProjectFeedAdd struct {
	Body      string   `json:"Body"`
	Notify    []string `json:"Notify,omitempty"`
	IsPrivate bool     `json:"IsPrivate"`
}

// wireTaskFeedAdd is the POST body for /api/projects/{p}/plans/{pl}/tasks/{t}/feed.
// Task-level feeds use the same "Comments" field as ticket feeds.
type wireTaskFeedAdd struct {
	Comments  string   `json:"Comments"`
	Notify    []string `json:"Notify,omitempty"`
	IsPrivate bool     `json:"IsPrivate"`
}

// parseTD parses TD's ISO-ish timestamp; returns zero time on empty/sentinel.
func parseTD(s string) time.Time {
	if s == "" || strings.HasPrefix(s, "0001-01-01") || strings.HasPrefix(s, "1900-01-01") {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05", // TD often omits the zone on plan/task dates
		"2006-01-02T15:04:05.999",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
