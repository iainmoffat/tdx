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

// NewFromDraft clones an existing draft into a new (profile, weekStart, name).
// SourceEntryIDs are intentionally cleared — the cloned draft is fresh, not a
// snapshot of remote state. Provenance records the shift in days from src.
func (s *Service) NewFromDraft(profile string, weekStart time.Time, name string,
	srcProfile string, srcWeekStart time.Time, srcName string) (domain.WeekDraft, error) {
	if name == "" {
		name = "default"
	}
	src, err := s.store.Load(srcProfile, srcWeekStart, srcName)
	if err != nil {
		return domain.WeekDraft{}, fmt.Errorf("load source: %w", err)
	}

	rows := make([]domain.DraftRow, len(src.Rows))
	for i, r := range src.Rows {
		cells := make([]domain.DraftCell, len(r.Cells))
		for j, c := range r.Cells {
			cells[j] = domain.DraftCell{
				Day:     c.Day,
				Hours:   c.Hours,
				PerCell: c.PerCell,
				// SourceEntryID intentionally omitted.
			}
		}
		rows[i] = domain.DraftRow{
			ID: r.ID, Label: r.Label, Target: r.Target, TimeType: r.TimeType,
			Description: r.Description, Billable: r.Billable, ResolverHints: r.ResolverHints,
			Cells: cells,
		}
	}

	shiftDays := int(weekStart.Sub(srcWeekStart).Hours() / 24)
	fromRef := fmt.Sprintf("%s/%s",
		srcWeekStart.In(domain.EasternTZ).Format("2006-01-02"), srcName)

	now := time.Now().UTC()
	d := domain.WeekDraft{
		SchemaVersion: 1,
		Profile:       profile,
		WeekStart:     weekStart,
		Name:          name,
		Provenance: domain.DraftProvenance{
			Kind:          domain.ProvenanceFromDraft,
			FromDraft:     fromRef,
			ShiftedByDays: shiftDays,
		},
		CreatedAt:  now,
		ModifiedAt: now,
		Rows:       rows,
	}
	if err := s.store.SaveNew(d); err != nil {
		return domain.WeekDraft{}, fmt.Errorf("new from draft: %w", err)
	}
	return d, nil
}
