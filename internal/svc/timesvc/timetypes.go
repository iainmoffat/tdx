package timesvc

import (
	"context"
	"fmt"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ListTimeTypes returns every time type visible to the authenticated user.
func (s *Service) ListTimeTypes(ctx context.Context, profileName string) ([]domain.TimeType, error) {
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireTimeType
	if err := client.DoJSON(ctx, "GET", "/TDWebApi/api/time/types", nil, &wire); err != nil {
		return nil, fmt.Errorf("list time types: %w", err)
	}
	out := make([]domain.TimeType, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeTimeType(w))
	}
	return out, nil
}

// decodeTimeType maps a TD wire struct into the idiomatic domain type.
// Extracted so every time-type-returning endpoint uses the same mapping.
func decodeTimeType(w wireTimeType) domain.TimeType {
	return domain.TimeType{
		ID:          w.ID,
		Name:        w.Name,
		Code:        w.Code,
		Description: w.HelpText,
		Billable:    w.IsBillable,
		Limited:     w.IsLimited,
		Active:      w.IsActive,
		IsTimeOff:   w.IsTimeOffTimeType,
	}
}

// TimeTypesForTarget returns the time types valid for a specific work item.
// Different TargetKind values hit different TD endpoints; see the
// TD reference for the full tree under /time/types/component/.
func (s *Service) TimeTypesForTarget(ctx context.Context, profileName string, target domain.Target) ([]domain.TimeType, error) {
	path, err := componentPathFor(target)
	if err != nil {
		return nil, err
	}
	client, err := s.clientFor(profileName)
	if err != nil {
		return nil, err
	}
	var wire []wireTimeType
	if err := client.DoJSON(ctx, "GET", path, nil, &wire); err != nil {
		return nil, fmt.Errorf("time types for %s: %w", target.Kind, err)
	}
	out := make([]domain.TimeType, 0, len(wire))
	for _, w := range wire {
		out = append(out, decodeTimeType(w))
	}
	return out, nil
}

// componentPathFor builds the /TDWebApi/api/time/types/component/... URL
// for a given target. Returns ErrUnsupportedTargetKind for kinds TD does
// not expose a component endpoint for (e.g., portfolio).
func componentPathFor(target domain.Target) (string, error) {
	switch target.Kind {
	case domain.TargetTicket:
		return fmt.Sprintf("/TDWebApi/api/time/types/component/app/%d/ticket/%d",
			target.AppID, target.ItemID), nil
	case domain.TargetTicketTask:
		return fmt.Sprintf("/TDWebApi/api/time/types/component/app/%d/ticket/%d/task/%d",
			target.AppID, target.ItemID, target.TaskID), nil
	case domain.TargetProject:
		return fmt.Sprintf("/TDWebApi/api/time/types/component/project/%d", target.ItemID), nil
	case domain.TargetProjectTask:
		// PlanID lives in Target.ItemID (set by decodeTarget for
		// componentTaskTime); TaskID in Target.TaskID; ProjectID in
		// Target.ProjectID. Verified against test tenant 2026-04-29.
		return fmt.Sprintf("/TDWebApi/api/time/types/component/project/%d/plan/%d/task/%d",
			target.ProjectID, target.ItemID, target.TaskID), nil
	case domain.TargetProjectIssue:
		return fmt.Sprintf("/TDWebApi/api/time/types/component/project/%d/issue/%d",
			target.ItemID, target.TaskID), nil
	case domain.TargetWorkspace:
		return fmt.Sprintf("/TDWebApi/api/time/types/component/workspace/%d", target.ItemID), nil
	case domain.TargetTimeOff:
		return fmt.Sprintf("/TDWebApi/api/time/types/component/timeoff/%d", target.ItemID), nil
	case domain.TargetRequest:
		return fmt.Sprintf("/TDWebApi/api/time/types/component/request/%d", target.ItemID), nil
	default:
		return "", fmt.Errorf("%w: %s", domain.ErrUnsupportedTargetKind, target.Kind)
	}
}
