package timesvc

import (
	"context"
	"fmt"

	"github.com/ipm/tdx/internal/domain"
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
	}
}
