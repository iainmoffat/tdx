package draftsvc

import (
	"fmt"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// Copy clones src into a new draft at dst. If src and dst share the same
// (profile, weekStart), the pulled-snapshot sibling is also copied (the
// watermark remains meaningful). Cross-date copies clear sourceEntryIDs and
// skip the pulled-snapshot copy.
func (s *Service) Copy(srcProfile string, srcWeekStart time.Time, srcName string,
	dstProfile string, dstWeekStart time.Time, dstName string) (domain.WeekDraft, error) {
	if dstName == "" {
		dstName = "default"
	}
	src, err := s.store.Load(srcProfile, srcWeekStart, srcName)
	if err != nil {
		return domain.WeekDraft{}, fmt.Errorf("copy: load source: %w", err)
	}

	sameWeek := srcWeekStart.Equal(dstWeekStart) && srcProfile == dstProfile

	rows := make([]domain.DraftRow, len(src.Rows))
	for i, r := range src.Rows {
		cells := make([]domain.DraftCell, len(r.Cells))
		for j, c := range r.Cells {
			nc := c
			if !sameWeek {
				nc.SourceEntryID = 0
			}
			cells[j] = nc
		}
		nr := r
		nr.Cells = cells
		rows[i] = nr
	}

	shiftDays := int(dstWeekStart.Sub(srcWeekStart).Hours() / 24)
	fromRef := fmt.Sprintf("%s/%s",
		srcWeekStart.In(domain.EasternTZ).Format("2006-01-02"), srcName)

	now := time.Now().UTC()
	dst := domain.WeekDraft{
		SchemaVersion: 1,
		Profile:       dstProfile,
		WeekStart:     dstWeekStart,
		Name:          dstName,
		Notes:         src.Notes,
		Tags:          src.Tags,
		Provenance: domain.DraftProvenance{
			Kind:          domain.ProvenanceFromDraft,
			FromDraft:     fromRef,
			ShiftedByDays: shiftDays,
		},
		CreatedAt:  now,
		ModifiedAt: now,
		Rows:       rows,
	}
	if err := s.store.SaveNew(dst); err != nil {
		return domain.WeekDraft{}, fmt.Errorf("copy: %w", err)
	}
	if sameWeek {
		if pulled, err := s.store.LoadPulledSnapshot(srcProfile, srcWeekStart, srcName); err == nil {
			pulled.Name = dstName
			if err := s.store.SavePulledSnapshot(pulled); err != nil {
				return domain.WeekDraft{}, fmt.Errorf("copy pulled snapshot: %w", err)
			}
		}
	}
	return dst, nil
}
