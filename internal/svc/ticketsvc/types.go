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
	StatusIDs          []int    `json:"StatusIDs,omitempty"`
	ResponsibilityUids []string `json:"ResponsibilityUids,omitempty"`
	RequestorUids      []string `json:"RequestorUids,omitempty"`
	AccountIDs         []int    `json:"AccountIDs,omitempty"`
	SearchText         string   `json:"SearchText,omitempty"`
	MaxResults         int      `json:"MaxResults,omitempty"`
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
