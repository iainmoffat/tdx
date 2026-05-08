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
type wireTicketStatus struct {
	ID          int     `json:"ID"`
	Name        string  `json:"Name"`
	IsActive    bool    `json:"IsActive"`
	Order       float64 `json:"Order"`
	IsDefault   bool    `json:"IsDefault"`
	StatusClass int     `json:"StatusClass"` // TD enum; 6 = Closed (verify live in Task 19)
}

// wireTicketType matches GET /TDWebApi/api/{appId}/tickets/types rows.
type wireTicketType struct {
	ID          int    `json:"ID"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	IsActive    bool   `json:"IsActive"`
}

// wireTicket matches GET /TDWebApi/api/{appId}/tickets/{id} and rows in
// POST /TDWebApi/api/{appId}/tickets/search responses.
type wireTicket struct {
	ID                  int      `json:"ID"`
	AppID               int      `json:"AppID"`
	Title               string   `json:"Title"`
	Description         string   `json:"Description"`
	StatusID            int      `json:"StatusID"`
	StatusName          string   `json:"StatusName"`
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
// Per the spec, only IsOpen/MaxResults are reliably honored on the live
// tenant — the CLI may still send other filters and post-filter client-side
// in later tasks; we send what TD documents and treat extra-filter
// fidelity as a best-effort.
type wireTicketSearch struct {
	StatusIDs          []int    `json:"StatusIDs,omitempty"`
	ResponsibilityUids []string `json:"ResponsibilityUids,omitempty"`
	RequestorUids      []string `json:"RequestorUids,omitempty"`
	AccountIDs         []int    `json:"AccountIDs,omitempty"`
	SearchText         string   `json:"SearchText,omitempty"`
	IsOpen             *bool    `json:"IsOpen,omitempty"`
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
