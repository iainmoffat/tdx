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

// NewFromTemplate creates a draft seeded from a template's rows.
// Cells are placed on weekdays where the template's WeekHours has non-zero hours.
func (s *Service) NewFromTemplate(profile string, weekStart time.Time, name string, tmpl domain.Template) (domain.WeekDraft, error) {
	if name == "" {
		name = "default"
	}
	rows := make([]domain.DraftRow, 0, len(tmpl.Rows))
	for i, tr := range tmpl.Rows {
		cells := make([]domain.DraftCell, 0, 7)
		for d := time.Sunday; d <= time.Saturday; d++ {
			h := tr.Hours.ForDay(d)
			if h == 0 {
				continue
			}
			cells = append(cells, domain.DraftCell{Day: d, Hours: h})
		}
		id := tr.ID
		if id == "" {
			id = fmt.Sprintf("row-%02d", i+1)
		}
		rows = append(rows, domain.DraftRow{
			ID:            id,
			Label:         tr.Label,
			Target:        tr.Target,
			TimeType:      tr.TimeType,
			Description:   tr.Description,
			Billable:      tr.Billable,
			ResolverHints: tr.ResolverHints,
			Cells:         cells,
		})
	}
	now := time.Now().UTC()
	d := domain.WeekDraft{
		SchemaVersion: 1,
		Profile:       profile,
		WeekStart:     weekStart,
		Name:          name,
		Provenance: domain.DraftProvenance{
			Kind:         domain.ProvenanceFromTemplate,
			FromTemplate: tmpl.Name,
		},
		CreatedAt:  now,
		ModifiedAt: now,
		Rows:       rows,
	}
	if err := s.store.SaveNew(d); err != nil {
		return domain.WeekDraft{}, fmt.Errorf("new from template: %w", err)
	}
	return d, nil
}
