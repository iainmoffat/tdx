package domain

import "time"

// TicketApp represents a TD ticketing application. A tenant has multiple
// ticket apps (service desk, project management, departmental). Every
// ticket-API call is scoped to one app via {appId} in the path.
type TicketApp struct {
	ID          int
	Name        string
	Description string
	Active      bool
	AppType     string // e.g. "TDNext", "TDTickets" — informational
}

// TicketStatus is one entry in an app's status workflow.
type TicketStatus struct {
	ID        int
	Name      string
	IsClosed  bool
	IsDefault bool
	Order     float64
}

// TicketType categorizes tickets within an app.
type TicketType struct {
	ID          int
	Name        string
	Description string
	Active      bool
}

// TicketGroup is a TD responsibility group (a team that can be assigned
// tickets). Groups exist tenant-wide and can serve multiple ticket apps;
// the search endpoint filters by ResponsibilityGroupIDs (server-side honored).
type TicketGroup struct {
	ID     int
	Name   string
	Active bool
}

// Ticket is one row from POST /tickets/search (partial; IsFull=false) or
// GET /tickets/{id} (full; IsFull=true).
type Ticket struct {
	ID               int
	AppID            int
	Title            string
	Description      string
	StatusID         int
	StatusName       string
	TypeID           int
	TypeName         string
	PriorityID       int
	PriorityName     string
	AccountID        int
	AccountName      string
	ResponsibleUID   string
	ResponsibleName  string
	RequestorUID     string
	RequestorName    string
	CreatedDate      time.Time
	ModifiedDate     time.Time
	EstimatedMinutes int
	ActualMinutes    int
	Tags             []string
	IsFull           bool
}

// TicketFeedEntry is a single feed row (comment, status change, etc).
type TicketFeedEntry struct {
	ID         int
	AuthorUID  string
	AuthorName string
	CreatedAt  time.Time
	Body       string
	IsPrivate  bool
	EventKind  string // "comment" | "statusChange" | "assignment" | "task" | etc.
}

// TicketSearchFilter drives POST /tickets/search.
type TicketSearchFilter struct {
	AppID                  int
	StatusIDs              []int
	AssigneeUIDs           []string
	RequestorUIDs          []string
	AccountIDs             []int
	ResponsibilityGroupIDs []int
	Text                   string
	IncludeClosed          bool
	Limit                  int
}

// TicketSavedSearch is one row from GET /tickets/searches.
type TicketSavedSearch struct {
	ID          int
	Name        string
	OwnerUID    string
	OwnerName   string
	Description string
}

// TicketTask is one task on a ticket. Tasks track sub-work with their own
// PercentComplete and (optionally) ResponsibleUid/Group. Time entries can
// target a ticket task via Target{Kind: TargetTicketTask, ItemID: ticketID,
// TaskID: taskID}.
type TicketTask struct {
	ID                   int
	TicketID             int
	Title                string
	Description          string
	Active               bool
	PercentComplete      int
	EstimatedMinutes     int
	ActualMinutes        int
	StartDate            time.Time
	EndDate              time.Time
	CreatedDate          time.Time
	CreatedName          string
	ModifiedDate         time.Time
	CompletedDate        time.Time
	CompletedName        string
	ResponsibleUID       string
	ResponsibleName      string
	ResponsibleGroupID   int
	ResponsibleGroupName string
	Order                int
}
