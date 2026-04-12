package domain

import "time"

// EntryInput holds the fields required to create a new time entry.
type EntryInput struct {
	UserUID     string
	Date        time.Time
	Minutes     int
	TimeTypeID  int
	Billable    bool
	Target      Target
	ProjectID   int // wire ProjectID for projectTask/projectIssue; 0 for others
	Description string
}

// EntryUpdate holds optional fields for updating an existing time entry.
// Nil pointer fields are left unchanged.
type EntryUpdate struct {
	Date        *time.Time
	Minutes     *int
	TimeTypeID  *int
	Billable    *bool
	Description *string
}

// IsEmpty returns true if no fields are set for update.
func (u EntryUpdate) IsEmpty() bool {
	return u.Date == nil && u.Minutes == nil && u.TimeTypeID == nil && u.Billable == nil && u.Description == nil
}
