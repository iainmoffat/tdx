package ticketsvc

// wireApp matches one row in the response from
// GET /TDWebApi/api/applications. The endpoint returns all platform
// applications; ListApps filters to ticketing apps via AppClass containing
// "Ticket".
type wireApp struct {
	ID          int    `json:"AppID"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	Active      bool   `json:"Active"`
	Type        string `json:"Type"`
	AppClass    string `json:"AppClass"`
}

// wireTicketStatus matches GET /TDWebApi/api/{appId}/tickets/statuses rows.
//
// StatusClass enum (live-verified on UFL 2026-05-08):
//
//	1 = New, 2 = InProcess, 3 = Completed (Resolved/Closed),
//	4 = Cancelled, 5 = OnHold
//
// Closed-state semantics: classes 3 (Completed) and 4 (Cancelled) are terminal.
type wireTicketStatus struct {
	ID          int     `json:"ID"`
	Name        string  `json:"Name"`
	IsActive    bool    `json:"IsActive"`
	Order       float64 `json:"Order"`
	IsDefault   bool    `json:"IsDefault"`
	StatusClass int     `json:"StatusClass"`
}

// wireTicketType matches GET /TDWebApi/api/{appId}/tickets/types rows.
type wireTicketType struct {
	ID          int    `json:"ID"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	IsActive    bool   `json:"IsActive"`
}

// wireTicket matches GET /TDWebApi/api/{appId}/tickets/{id} and rows in
// POST /TDWebApi/api/{appId}/tickets/search responses. StatusClass is
// included so client-side open-only filtering can be applied (TD's IsOpen
// filter on the search endpoint is silently ignored on UFL).
type wireTicket struct {
	ID                  int      `json:"ID"`
	AppID               int      `json:"AppID"`
	Title               string   `json:"Title"`
	Description         string   `json:"Description"`
	StatusID            int      `json:"StatusID"`
	StatusName          string   `json:"StatusName"`
	StatusClass         int      `json:"StatusClass"`
	TypeID              int      `json:"TypeID"`
	TypeName            string   `json:"TypeName"`
	PriorityID          int      `json:"PriorityID"`
	PriorityName        string   `json:"PriorityName"`
	AccountID           int      `json:"AccountID"`
	AccountName         string   `json:"AccountName"`
	ResponsibleUid      string   `json:"ResponsibleUid"`
	ResponsibleFullName string   `json:"ResponsibleFullName"`
	RequestorUid        string   `json:"RequestorUid"`
	RequestorName       string   `json:"RequestorName"`
	CreatedDate         string   `json:"CreatedDate"`
	ModifiedDate        string   `json:"ModifiedDate"`
	EstimatedMinutes    int      `json:"EstimatedMinutes"`
	ActualMinutes       int      `json:"ActualMinutes"`
	Tags                []string `json:"Tags"`
}

// wireTicketSearch is the request body for POST /tickets/search.
//
// Live-verified on UFL 2026-05-08: only StatusIDs/MaxResults/ResponsibilityUids/
// RequestorUids are reliably honored. The IsOpen field that TD documents on
// this endpoint is silently ignored — open-only filtering is done client-side
// by SearchTickets using the StatusClass field returned on each row.
type wireTicketSearch struct {
	StatusIDs              []int    `json:"StatusIDs,omitempty"`
	ResponsibilityUids     []string `json:"ResponsibilityUids,omitempty"`
	ResponsibilityGroupIDs []int    `json:"ResponsibilityGroupIDs,omitempty"`
	RequestorUids          []string `json:"RequestorUids,omitempty"`
	AccountIDs             []int    `json:"AccountIDs,omitempty"`
	SearchText             string   `json:"SearchText,omitempty"`
	MaxResults             int      `json:"MaxResults,omitempty"`
}

// PatchOp is one JSON-Patch operation. Exported so the CLI layer
// can construct ops directly without importing wire-private types.
type PatchOp struct {
	Op    string      `json:"op"`   // "replace", "add", etc.
	Path  string      `json:"path"` // "/StatusID", "/ResponsibleUid", etc.
	Value interface{} `json:"value"`
}

// wireFeedEntry matches a row in GET /tickets/{id}/feed and the response
// from POST /tickets/{id}/feed.
type wireFeedEntry struct {
	ID              int    `json:"ID"`
	CreatedUid      string `json:"CreatedUid"`
	CreatedFullName string `json:"CreatedFullName"`
	CreatedDate     string `json:"CreatedDate"`
	Body            string `json:"Body"`
	IsPrivate       bool   `json:"IsPrivate"`
	UpdateType      int    `json:"UpdateType"` // TD enum; verify live in Task 19
}

// wireFeedAdd is the request body for POST /tickets/{id}/feed.
type wireFeedAdd struct {
	Comments  string   `json:"Comments"`
	Notify    []string `json:"Notify,omitempty"`
	IsPrivate bool     `json:"IsPrivate"`
}

// wireSavedSearch matches a row in GET /tickets/searches.
// Live-verified on UFL 2026-05-08: owner fields are CreatedUID/CreatedFullName,
// and there is no Description field on saved searches in this app type.
type wireSavedSearch struct {
	ID              int    `json:"ID"`
	Name            string `json:"Name"`
	CreatedUID      string `json:"CreatedUID"`
	CreatedFullName string `json:"CreatedFullName"`
}

// wireRequestPage is TD's required pagination object for saved-search results.
type wireRequestPage struct {
	PageIndex int `json:"PageIndex"`
	PageSize  int `json:"PageSize"`
}

// wireSavedSearchOptions is the request body for POST /tickets/searches/{id}/results.
// Page is required (the endpoint 400s without it).
type wireSavedSearchOptions struct {
	Page wireRequestPage `json:"Page"`
}

// wireSavedSearchResults is the response wrapper from POST /tickets/searches/{id}/results.
// Unlike POST /tickets/search (flat list), saved-search results come paginated.
type wireSavedSearchResults struct {
	Data             []wireTicket `json:"Data"`
	TotalCount       int          `json:"TotalCount"`
	CurrentPageIndex int          `json:"CurrentPageIndex"`
	PageSize         int          `json:"PageSize"`
}

// wireGroup matches a row in POST /TDWebApi/api/groups/search.
// PlatformApplications is included in the response but not surfaced
// to consumers in v0.16.1.
type wireGroup struct {
	ID                   int           `json:"ID"`
	Name                 string        `json:"Name"`
	IsActive             bool          `json:"IsActive"`
	Description          string        `json:"Description,omitempty"`
	ExternalID           string        `json:"ExternalID,omitempty"`
	PlatformApplications []interface{} `json:"PlatformApplications,omitempty"`
}

// wireTicketTask matches GET /TDWebApi/api/{appId}/tickets/{ticketID}/tasks
// rows and GET /TDWebApi/api/{appId}/tickets/{ticketID}/tasks/{id}.
//
// Live-verified on UFL 2026-05-08: dates use TD's standard format and
// CompletedDate may be "0001-01-01T00:00:00" when unset (parseTDTime
// returns zero time for that). ResponsibleUid is null (empty string)
// when unassigned at the individual level — group assignment uses
// ResponsibleGroupID/Name instead.
type wireTicketTask struct {
	ID                   int    `json:"ID"`
	TicketID             int    `json:"TicketID"`
	Title                string `json:"Title"`
	Description          string `json:"Description"`
	IsActive             bool   `json:"IsActive"`
	PercentComplete      int    `json:"PercentComplete"`
	EstimatedMinutes     int    `json:"EstimatedMinutes"`
	ActualMinutes        int    `json:"ActualMinutes"`
	StartDate            string `json:"StartDate,omitempty"`
	EndDate              string `json:"EndDate,omitempty"`
	CreatedDate          string `json:"CreatedDate"`
	CreatedFullName      string `json:"CreatedFullName"`
	ModifiedDate         string `json:"ModifiedDate"`
	CompletedDate        string `json:"CompletedDate"`
	CompletedFullName    string `json:"CompletedFullName"`
	ResponsibleUid       string `json:"ResponsibleUid"`
	ResponsibleFullName  string `json:"ResponsibleFullName"`
	ResponsibleGroupID   int    `json:"ResponsibleGroupID"`
	ResponsibleGroupName string `json:"ResponsibleGroupName"`
	Order                int    `json:"Order"`
}

// wireTaskFeedUpdate is the request body for
// POST /TDWebApi/api/{appId}/tickets/{ticketID}/tasks/{taskID}/feed.
//
// HoursWorked is informational only — it does NOT create a time entry
// or update the task's ActualMinutes. Use a separate time entry
// (`tdx ticket task log`) for real time tracking.
//
// PercentComplete is *int because 0 is a valid value (means "set to 0%
// complete"); nil means "don't send PercentComplete in the body."
type wireTaskFeedUpdate struct {
	Comments        string   `json:"Comments,omitempty"`
	PercentComplete *int     `json:"PercentComplete,omitempty"`
	HoursWorked     float64  `json:"HoursWorked,omitempty"`
	IsPrivate       bool     `json:"IsPrivate,omitempty"`
	Notify          []string `json:"Notify,omitempty"`
}
