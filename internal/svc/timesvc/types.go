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
func (t *tdTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		return nil
	}

	// Try RFC3339 forms first (with optional fractional seconds and zone).
	if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
		t.Time = parsed
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, s); err == nil {
		t.Time = parsed
		return nil
	}

	// Fall back to no-zone wall-clock forms, with or without fractional
	// seconds. Interpret as EasternTZ (TD tenant-local time).
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
	} {
		if parsed, err := time.ParseInLocation(layout, s, domain.EasternTZ); err == nil {
			t.Time = parsed
			return nil
		}
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
