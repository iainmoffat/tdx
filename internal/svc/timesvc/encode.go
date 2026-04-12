package timesvc

import (
	"fmt"

	"github.com/ipm/tdx/internal/domain"
)

// encodeTarget maps a domain.Target to wire fields on a wireTimeEntryWrite.
// projectID is the wire ProjectID required for projectTask and projectIssue kinds.
func encodeTarget(t domain.Target, projectID int, w *wireTimeEntryWrite) error {
	switch t.Kind {
	case domain.TargetTicket:
		w.Component = componentTicketTime
		w.TicketID = t.ItemID
		w.AppID = t.AppID
	case domain.TargetTicketTask:
		w.Component = componentTicketTaskTime
		w.TicketID = t.ItemID
		w.ItemID = t.TaskID
		w.AppID = t.AppID
	case domain.TargetProject:
		w.Component = componentProjectTime
		w.ProjectID = t.ItemID
	case domain.TargetProjectTask:
		w.Component = componentTaskTime
		w.ProjectID = projectID
		w.PlanID = t.ItemID
		w.ItemID = t.TaskID
	case domain.TargetProjectIssue:
		w.Component = componentIssueTime
		w.ProjectID = projectID
		w.ItemID = t.ItemID
	case domain.TargetWorkspace:
		w.Component = componentWorkspaceTime
		w.ProjectID = t.ItemID
	case domain.TargetTimeOff:
		w.Component = componentTimeOff
		w.ProjectID = t.ItemID
	case domain.TargetPortfolio:
		w.Component = componentPortfolioTime
		w.PortfolioID = t.ItemID
		w.ItemID = t.ItemID
	default:
		return fmt.Errorf("%w: %s", domain.ErrUnsupportedTargetKind, t.Kind)
	}
	return nil
}

// encodeEntryWrite builds a wireTimeEntryWrite from a domain.EntryInput.
func encodeEntryWrite(input domain.EntryInput) (wireTimeEntryWrite, error) {
	w := wireTimeEntryWrite{
		Uid:         input.UserUID,
		TimeDate:    input.Date.Format("2006-01-02T00:00:00"),
		Minutes:     float64(input.Minutes),
		TimeTypeID:  input.TimeTypeID,
		Description: input.Description,
		Billable:    input.Billable,
	}
	if err := encodeTarget(input.Target, input.ProjectID, &w); err != nil {
		return wireTimeEntryWrite{}, err
	}
	return w, nil
}
