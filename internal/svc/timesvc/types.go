package timesvc

import "time"

// All types in this file are private wire structs that match TeamDynamix's
// JSON exactly. They are mapped to internal/domain types inside each method.
// Do NOT expose any of these from the package.

// TD TimeEntryComponent enum values (from the TD Web API reference).
const (
	componentProjectTime      = 1
	componentTaskTime         = 2
	componentIssueTime        = 3
	componentTicketTime       = 9
	componentTimeOff          = 17
	componentPortfolioTime    = 23
	componentTicketTaskTime   = 25
	componentWorkspaceTime    = 45
	componentPortfolioIssTime = 83
)

// TD TimeStatus enum values.
const (
	tdStatusNoStatus  = 0
	tdStatusSubmitted = 1
	tdStatusRejected  = 2
	tdStatusApproved  = 3
)

// wireTimeType matches GET /TDWebApi/api/time/types (and siblings).
type wireTimeType struct {
	ID                  int    `json:"ID"`
	Name                string `json:"Name"`
	Code                string `json:"Code"`
	GLAccount           string `json:"GLAccount"`
	HelpText            string `json:"HelpText"`
	DefaultLimitMinutes int    `json:"DefaultLimitMinutes"`
	IsBillable          bool   `json:"IsBillable"`
	IsCapitalized       bool   `json:"IsCapitalized"`
	IsLimited           bool   `json:"IsLimited"`
	IsActive            bool   `json:"IsActive"`
	IsTimeOffTimeType   bool   `json:"IsTimeOffTimeType"`
}

// wireTimeEntry matches GET /TDWebApi/api/time/{id} and the response body
// of POST /TDWebApi/api/time/search (which is a TimeEntry[]).
type wireTimeEntry struct {
	TimeID        int       `json:"TimeID"`
	ItemID        int       `json:"ItemID"`
	ItemTitle     string    `json:"ItemTitle"`
	AppID         int       `json:"AppID"`
	AppName       string    `json:"AppName"`
	Component     int       `json:"Component"`
	TicketID      int       `json:"TicketID"`
	ProjectID     int       `json:"ProjectID"`
	ProjectName   string    `json:"ProjectName"`
	PlanID        int       `json:"PlanID"`
	PortfolioID   int       `json:"PortfolioID"`
	PortfolioName string    `json:"PortfolioName"`
	TimeDate      time.Time `json:"TimeDate"`
	Minutes       float64   `json:"Minutes"`
	Description   string    `json:"Description"`
	TimeTypeID    int       `json:"TimeTypeID"`
	TimeTypeName  string    `json:"TimeTypeName"`
	Billable      bool      `json:"Billable"`
	Limited       bool      `json:"Limited"`
	Uid           string    `json:"Uid"`
	Status        int       `json:"Status"`
	StatusDate    time.Time `json:"StatusDate"`
	CreatedDate   time.Time `json:"CreatedDate"`
	ModifiedDate  time.Time `json:"ModifiedDate"`
}

// wireTimeReport matches GET /TDWebApi/api/time/report/{date}.
type wireTimeReport struct {
	ID                 int             `json:"ID"`
	PeriodStartDate    time.Time       `json:"PeriodStartDate"`
	PeriodEndDate      time.Time       `json:"PeriodEndDate"`
	Status             int             `json:"Status"`
	Times              []wireTimeEntry `json:"Times"`
	TimeReportUid      string          `json:"TimeReportUid"`
	UserFullName       string          `json:"UserFullName"`
	MinutesBillable    int             `json:"MinutesBillable"`
	MinutesNonBillable int             `json:"MinutesNonBillable"`
	MinutesTotal       int             `json:"MinutesTotal"`
	TimeEntriesCount   int             `json:"TimeEntriesCount"`
}

// wireTimeSearch is the request body for POST /TDWebApi/api/time/search.
// All fields are optional on the server side; send only the ones the caller
// actually wants filtered.
type wireTimeSearch struct {
	EntryDateFrom  *time.Time `json:"EntryDateFrom,omitempty"`
	EntryDateTo    *time.Time `json:"EntryDateTo,omitempty"`
	TimeTypeIDs    []int      `json:"TimeTypeIDs,omitempty"`
	TicketIDs      []int      `json:"TicketIDs,omitempty"`
	ApplicationIDs []int      `json:"ApplicationIDs,omitempty"`
	PersonUIDs     []string   `json:"PersonUIDs,omitempty"`
	MaxResults     int        `json:"MaxResults,omitempty"`
}
