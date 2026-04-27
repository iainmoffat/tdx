package draftsvc

import (
	"fmt"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// NewBlank creates an empty dated draft. Refuses if (profile, weekStart, name)
// already exists.
func (s *Service) NewBlank(profile string, weekStart time.Time, name string) (domain.WeekDraft, error) {
	if name == "" {
		name = "default"
	}
	now := time.Now().UTC()
	d := domain.WeekDraft{
		SchemaVersion: 1,
		Profile:       profile,
		WeekStart:     weekStart,
		Name:          name,
		Provenance:    domain.DraftProvenance{Kind: domain.ProvenanceBlank},
		CreatedAt:     now,
		ModifiedAt:    now,
	}
	if err := s.store.SaveNew(d); err != nil {
		return domain.WeekDraft{}, fmt.Errorf("new blank: %w", err)
	}
	return d, nil
}
