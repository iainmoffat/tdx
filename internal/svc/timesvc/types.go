package timesvc

import (
	"fmt"
	"strings"
	"time"

	"github.com/ipm/tdx/internal/domain"
)

// All types in this file are private wire structs that match TeamDynamix's
// JSON exactly. They are mapped to internal/domain types inside each method.
// Do NOT expose any of these from the package.

// tdTime is a time.Time that tolerates TD's idiosyncratic JSON encoding.
// TD returns timestamps in two formats:
//   - RFC3339 with Z (or numeric offset), e.g. "2026-04-06T00:00:00Z" — used
//     for date-only fields like TimeDate and PeriodStartDate.
//   - Local wall-clock with no zone, e.g. "2026-04-03T15:22:01.607" — used
//     for system timestamps like CreatedDate, ModifiedDate, StatusDate. The
//     no-zone form is interpreted as TD tenant-local time (EasternTZ).
//
// Discovered during the Phase 2 manual walkthrough when GET /api/time/search
// returned a CreatedDate without a zone suffix and Go's default time.Time
// unmarshal rejected it.
type tdTime struct {
	time.Time
}

// UnmarshalJSON parses a TD timestamp in either RFC3339 form (with zone)
// or wall-clock form (no zone, interpreted as EasternTZ). Empty string and
// JSON null both produce a zero-value tdTime.
//
// Note on layout choice: Go's time.Parse accepts optional fractional seconds
// on the INPUT regardless of whether the layout string includes them. That
// means time.RFC3339 alone handles both "2026-04-06T00:00:00Z" and
// "2026-04-06T15:22:01.607Z", and the no-zone layout below handles both
// "2026-04-03T15:22:01" and "2026-04-03T15:22:01.607". Two attempts cover
// all four observed formats — zoned vs. no-zone is the only real split.
func (t *tdTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		return nil
	}

	// Try RFC3339 first (handles Z and numeric offsets, with or without
	// fractional seconds).
	if parsed, err := time.Parse(time.RFC3339, s); err == nil {
		t.Time = parsed
		return nil
	}

	// Fall back to no-zone wall-clock form, interpreted as EasternTZ
	// (TD tenant-local time). Handles inputs with or without fractional
	// seconds because Go's parser auto-accepts them.
	if parsed, err := time.ParseInLocation("2006-01-02T15:04:05", s, domain.EasternTZ); err == nil {
		t.Time = parsed
		return nil
	}

	return fmt.Errorf("tdTime: cannot parse %q", s)
}

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
	TimeDate      tdTime  `json:"TimeDate"`
	Minutes       float64 `json:"Minutes"`
	Description   string  `json:"Description"`
	TimeTypeID    int     `json:"TimeTypeID"`
	TimeTypeName  string  `json:"TimeTypeName"`
	Billable      bool    `json:"Billable"`
	Limited       bool    `json:"Limited"`
	Uid           string  `json:"Uid"`
	Status        int     `json:"Status"`
	StatusDate    tdTime  `json:"StatusDate"`
	CreatedDate   tdTime  `json:"CreatedDate"`
	ModifiedDate  tdTime  `json:"ModifiedDate"`
}

// wireTimeReport matches GET /TDWebApi/api/time/report/{date}.
type wireTimeReport struct {
	ID                 int             `json:"ID"`
	PeriodStartDate    tdTime          `json:"PeriodStartDate"`
	PeriodEndDate      tdTime          `json:"PeriodEndDate"`
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

// wireTimeEntryWrite is the request body for POST/PUT /api/time.
// Field names verified against live UFL tenant during Phase 3 Step 0 probing.
type wireTimeEntryWrite struct {
	TimeID      int     `json:"TimeID,omitempty"`
	Uid         string  `json:"Uid"`
	TimeDate    string  `json:"TimeDate"`
	Minutes     float64 `json:"Minutes"`
	TimeTypeID  int     `json:"TimeTypeID"`
	Component   int     `json:"Component"`
	TicketID    int     `json:"TicketID,omitempty"`
	ProjectID   int     `json:"ProjectID,omitempty"`
	PlanID      int     `json:"PlanID,omitempty"`
	PortfolioID int     `json:"PortfolioID,omitempty"`
	ItemID      int     `json:"ItemID,omitempty"`
	AppID       int     `json:"AppID,omitempty"`
	Description string  `json:"Description"`
	Billable    bool    `json:"Billable"`
}

// wireBulkResult is the response from batch POST /api/time and POST /api/time/delete.
// Shape verified against live UFL tenant during Phase 3 Step 0 probing.
type wireBulkResult struct {
	Succeeded []wireBulkSuccess `json:"Succeeded"`
	Failed    []wireBulkFailure `json:"Failed"`
}

type wireBulkSuccess struct {
	Index int `json:"Index"`
	ID    int `json:"ID"`
}

type wireBulkFailure struct {
	Index         int    `json:"Index"`
	TimeEntryID   int    `json:"TimeEntryID"`
	ErrorMessage  string `json:"ErrorMessage"`
	ErrorCode     int    `json:"ErrorCode"`
	ErrorCodeName string `json:"ErrorCodeName"`
}
