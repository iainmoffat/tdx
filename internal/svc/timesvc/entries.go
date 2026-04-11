package timesvc

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/ipm/tdx/internal/domain"
	"github.com/ipm/tdx/internal/tdx"
)

// SearchEntries runs POST /TDWebApi/api/time/search with the given filter.
// Zero-value filter fields are omitted from the request body so TD does not
// apply spurious filtering. Limit=0 means "use TD's default" (1000).
func (s *Service) SearchEntries(ctx context.Context, profileName string, filter domain.EntryFilter) ([]domain.TimeEntry, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}

	req := wireTimeSearch{MaxResults: filter.Limit}
	if !filter.DateRange.From.IsZero() {
		from := filter.DateRange.From
		req.EntryDateFrom = &from
	}
	if !filter.DateRange.To.IsZero() {
		to := filter.DateRange.To
		req.EntryDateTo = &to
	}
	if filter.UserUID != "" {
		req.PersonUIDs = []string{filter.UserUID}
	}
	if filter.Target != nil {
		if filter.Target.AppID > 0 {
			req.ApplicationIDs = []int{filter.Target.AppID}
		}
		if filter.Target.Kind == domain.TargetTicket && filter.Target.ItemID > 0 {
			req.TicketIDs = []int{filter.Target.ItemID}
		}
	}
	if filter.TimeTypeID > 0 {
		req.TimeTypeIDs = []int{filter.TimeTypeID}
	}

	var wire []wireTimeEntry
	if err := client.DoJSON(ctx, "POST", "/TDWebApi/api/time/search", req, &wire); err != nil {
		return nil, fmt.Errorf("search entries: %w", err)
	}

	out := make([]domain.TimeEntry, 0, len(wire))
	for _, w := range wire {
		entry, err := decodeTimeEntry(w)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := s.resolveTimeTypeNames(ctx, profileName, out); err != nil {
		return nil, err
	}
	return out, nil
}

// decodeTimeEntry maps a TD wire entry into the idiomatic domain type.
// The TD Component enum discriminates which of the many ID fields are
// meaningful; the canonical mapping is in the Phase 2 plan header.
func decodeTimeEntry(w wireTimeEntry) (domain.TimeEntry, error) {
	target, err := decodeTarget(w)
	if err != nil {
		return domain.TimeEntry{}, err
	}
	return domain.TimeEntry{
		ID:      w.TimeID,
		UserUID: w.Uid,
		Target:  target,
		TimeType: domain.TimeType{
			ID:   w.TimeTypeID,
			Name: w.TimeTypeName,
		},
		Date:         timeDateToEasternMidnight(w.TimeDate.Time),
		Minutes:      int(w.Minutes),
		Description:  w.Description,
		Billable:     w.Billable,
		CreatedAt:    w.CreatedDate.Time,
		ModifiedAt:   w.ModifiedDate.Time,
		ReportStatus: decodeReportStatus(w.Status),
	}, nil
}

// decodeTarget picks the right TargetKind and ID fields based on the TD
// Component enum discriminator.
func decodeTarget(w wireTimeEntry) (domain.Target, error) {
	t := domain.Target{
		AppID:       w.AppID,
		DisplayName: w.ItemTitle,
	}
	switch w.Component {
	case componentTicketTime:
		t.Kind = domain.TargetTicket
		t.ItemID = w.TicketID
		if t.ItemID == 0 {
			t.ItemID = w.ItemID
		}
		t.DisplayRef = fmt.Sprintf("#%d", t.ItemID)
	case componentTicketTaskTime:
		t.Kind = domain.TargetTicketTask
		t.ItemID = w.TicketID
		t.TaskID = w.ItemID
		t.DisplayRef = fmt.Sprintf("#%d/task/%d", t.ItemID, t.TaskID)
	case componentProjectTime:
		t.Kind = domain.TargetProject
		t.ItemID = w.ProjectID
		if t.ItemID == 0 {
			t.ItemID = w.ItemID
		}
		if w.ProjectName != "" {
			t.DisplayName = w.ProjectName
		}
		t.DisplayRef = fmt.Sprintf("project/%d", t.ItemID)
	case componentTaskTime:
		t.Kind = domain.TargetProjectTask
		t.ItemID = w.PlanID
		t.TaskID = w.ItemID
		t.DisplayRef = fmt.Sprintf("plan/%d/task/%d", t.ItemID, t.TaskID)
	case componentIssueTime:
		t.Kind = domain.TargetProjectIssue
		t.ItemID = w.ItemID
		t.DisplayRef = fmt.Sprintf("issue/%d", t.ItemID)
	case componentTimeOff:
		t.Kind = domain.TargetTimeOff
		t.ItemID = w.ProjectID
		if t.ItemID == 0 {
			t.ItemID = w.ItemID
		}
		t.DisplayRef = "time-off"
	case componentPortfolioTime, componentPortfolioIssTime:
		t.Kind = domain.TargetPortfolio
		t.ItemID = w.PortfolioID
		if t.ItemID == 0 {
			t.ItemID = w.ItemID
		}
		if w.PortfolioName != "" {
			t.DisplayName = w.PortfolioName
		}
		t.DisplayRef = fmt.Sprintf("portfolio/%d", t.ItemID)
	case componentWorkspaceTime:
		t.Kind = domain.TargetWorkspace
		t.ItemID = w.ProjectID
		if t.ItemID == 0 {
			t.ItemID = w.ItemID
		}
		t.DisplayRef = fmt.Sprintf("workspace/%d", t.ItemID)
	default:
		return domain.Target{}, fmt.Errorf("%w: component %d",
			domain.ErrUnsupportedTargetKind, w.Component)
	}
	return t, nil
}

// decodeReportStatus maps TD's TimeStatus enum (int) to the domain enum.
func decodeReportStatus(s int) domain.ReportStatus {
	switch s {
	case tdStatusSubmitted:
		return domain.ReportSubmitted
	case tdStatusApproved:
		return domain.ReportApproved
	case tdStatusRejected:
		return domain.ReportRejected
	default:
		return domain.ReportOpen
	}
}

// resolveTimeTypeNames populates TimeType.Name (and the rest of the
// TimeType fields) on each entry by looking up the type by ID via
// ListTimeTypes. TD's /api/time/search and /api/time/report responses
// include TimeTypeID but not TimeTypeName, so this is a side join with
// the type catalog. Skips the lookup entirely if no entries need it.
func (s *Service) resolveTimeTypeNames(ctx context.Context, profileName string, entries []domain.TimeEntry) error {
	needLookup := false
	for _, e := range entries {
		if e.TimeType.ID > 0 && e.TimeType.Name == "" {
			needLookup = true
			break
		}
	}
	if !needLookup {
		return nil
	}

	types, err := s.ListTimeTypes(ctx, profileName)
	if err != nil {
		return fmt.Errorf("resolve time type names: %w", err)
	}
	byID := make(map[int]domain.TimeType, len(types))
	for _, t := range types {
		byID[t.ID] = t
	}
	for i := range entries {
		if entries[i].TimeType.ID > 0 && entries[i].TimeType.Name == "" {
			if t, ok := byID[entries[i].TimeType.ID]; ok {
				entries[i].TimeType = t
			}
		}
	}
	return nil
}

// GetEntry fetches a single time entry by ID. 404 → ErrEntryNotFound.
func (s *Service) GetEntry(ctx context.Context, profileName string, id int) (domain.TimeEntry, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return domain.TimeEntry{}, err
	}
	var wire wireTimeEntry
	path := fmt.Sprintf("/TDWebApi/api/time/%d", id)
	err = client.DoJSON(ctx, "GET", path, nil, &wire)
	if err != nil {
		var apiErr *tdx.APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return domain.TimeEntry{}, fmt.Errorf("%w: %d", domain.ErrEntryNotFound, id)
		}
		return domain.TimeEntry{}, fmt.Errorf("get entry: %w", err)
	}
	entry, err := decodeTimeEntry(wire)
	if err != nil {
		return entry, err
	}
	single := []domain.TimeEntry{entry}
	if err := s.resolveTimeTypeNames(ctx, profileName, single); err != nil {
		return domain.TimeEntry{}, err
	}
	return single[0], nil
}
