package domain

import (
	"encoding/json"
	"time"
)

// ReportStatus is the approval state of a weekly time report or the
// individual entries inside one.
type ReportStatus string

const (
	ReportOpen      ReportStatus = "open"
	ReportSubmitted ReportStatus = "submitted"
	ReportApproved  ReportStatus = "approved"
	ReportRejected  ReportStatus = "rejected"
)

// IsTerminal reports whether the status represents a finalized decision
// (approved or rejected) that cannot be further transitioned by the user.
func (s ReportStatus) IsTerminal() bool {
	return s == ReportApproved || s == ReportRejected
}

// DateRange is an inclusive [From, To] range. Both ends must be midnight
// in EasternTZ for Contains to work correctly.
type DateRange struct {
	From time.Time `json:"from" yaml:"from"`
	To   time.Time `json:"to" yaml:"to"`
}

// Contains reports whether t (in EasternTZ) falls within [From, To] inclusive.
func (r DateRange) Contains(t time.Time) bool {
	et := t.In(EasternTZ)
	return !et.Before(r.From) && !et.After(r.To)
}

// TimeEntry is a single logged time row from TD. All dates are stored as
// midnight in EasternTZ; JSON marshals the Date field as a plain YYYY-MM-DD
// string via the custom MarshalJSON below.
type TimeEntry struct {
	ID           int          `json:"id"`
	UserUID      string       `json:"userUID"`
	Target       Target       `json:"target"`
	TimeType     TimeType     `json:"timeType"`
	Date         time.Time    `json:"-"`
	Minutes      int          `json:"minutes"`
	Description  string       `json:"description"`
	Billable     bool         `json:"billable"`
	CreatedAt    time.Time    `json:"createdAt"`
	ModifiedAt   time.Time    `json:"modifiedAt"`
	ReportStatus ReportStatus `json:"reportStatus"`
}

// Hours returns Minutes / 60 as a float64 for rendering.
func (e TimeEntry) Hours() float64 { return float64(e.Minutes) / 60.0 }

// timeEntryJSON is the wire shape used by MarshalJSON to override the
// default time.Time encoding on Date. CreatedAt and ModifiedAt use pointers
// so omitempty suppresses zero values (which would otherwise render as
// "0001-01-01T00:00:00Z" and pollute output).
type timeEntryJSON struct {
	ID           int          `json:"id"`
	UserUID      string       `json:"userUID"`
	Target       Target       `json:"target"`
	TimeType     TimeType     `json:"timeType"`
	Date         string       `json:"date"`
	Minutes      int          `json:"minutes"`
	Hours        float64      `json:"hours"`
	Description  string       `json:"description"`
	Billable     bool         `json:"billable"`
	CreatedAt    *time.Time   `json:"createdAt,omitempty"`
	ModifiedAt   *time.Time   `json:"modifiedAt,omitempty"`
	ReportStatus ReportStatus `json:"reportStatus"`
}

// MarshalJSON emits Date as "YYYY-MM-DD" and adds a derived Hours field.
// Zero CreatedAt/ModifiedAt values are omitted from the output.
func (e TimeEntry) MarshalJSON() ([]byte, error) {
	j := timeEntryJSON{
		ID:           e.ID,
		UserUID:      e.UserUID,
		Target:       e.Target,
		TimeType:     e.TimeType,
		Date:         e.Date.Format("2006-01-02"),
		Minutes:      e.Minutes,
		Hours:        e.Hours(),
		Description:  e.Description,
		Billable:     e.Billable,
		ReportStatus: e.ReportStatus,
	}
	if !e.CreatedAt.IsZero() {
		j.CreatedAt = &e.CreatedAt
	}
	if !e.ModifiedAt.IsZero() {
		j.ModifiedAt = &e.ModifiedAt
	}
	return json.Marshal(j)
}

// EntryFilter is the search criteria passed to timesvc.SearchEntries.
// Zero values mean "no filter on this field". Limit=0 means "let the caller
// pick a default" (CLI default is 100).
type EntryFilter struct {
	DateRange  DateRange
	UserUID    string
	Target     *Target
	TimeTypeID int
	Limit      int
}
