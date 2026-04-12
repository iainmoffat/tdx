package tmplsvc

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ipm/tdx/internal/domain"
)

// rowKey is the composite key used to group time entries into template rows.
// Entries that share the same target identity, time type, and billable flag
// are folded into a single row.
type rowKey struct {
	kind     domain.TargetKind
	appID    int
	itemID   int
	taskID   int
	typeID   int
	billable bool
}

// rowAccumulator holds the in-progress state for one template row group.
type rowAccumulator struct {
	target       domain.Target
	timeType     domain.TimeType
	billable     bool
	hours        domain.WeekHours
	descriptions map[string]int // description → occurrence count
}

// addEntry folds a single TimeEntry into this accumulator.
func (a *rowAccumulator) addEntry(e domain.TimeEntry, weekStart time.Time) {
	h := float64(e.Minutes) / 60.0

	// Compute day index using calendar-date arithmetic (DST-safe).
	ey, em, ed := e.Date.Date()
	ry, rm, rd := weekStart.Date()
	entryDay := time.Date(ey, em, ed, 0, 0, 0, 0, time.UTC)
	refDay := time.Date(ry, rm, rd, 0, 0, 0, 0, time.UTC)
	dayIdx := int(entryDay.Sub(refDay).Hours() / 24)

	if dayIdx >= 0 && dayIdx < 7 {
		weekday := weekStart.AddDate(0, 0, dayIdx).Weekday()
		existing := a.hours.ForDay(weekday)
		a.hours.SetDay(weekday, existing+h)
	}

	if e.Description != "" {
		a.descriptions[e.Description]++
	}
}

// mostCommonDescription returns the description with the highest occurrence
// count; ties are broken in favour of the lexicographically smaller string
// so the result is deterministic.
func (a *rowAccumulator) mostCommonDescription() string {
	best := ""
	bestCount := 0
	for desc, count := range a.descriptions {
		if count > bestCount || (count == bestCount && desc < best) {
			best = desc
			bestCount = count
		}
	}
	return best
}

// Derive creates a new template from the live week that contains weekDate,
// fetching entries via the timesvc for the named profile. The template is
// saved to disk and returned.
//
// Errors:
//   - "already exists" when a template named templateName already exists.
//   - "no entries in week of…" when the fetched report has no time entries.
func (s *Service) Derive(ctx context.Context, profileName, templateName string, weekDate time.Time) (domain.Template, error) {
	// 1. Guard: template must not already exist.
	if s.store.Exists(templateName) {
		return domain.Template{}, fmt.Errorf("template %q already exists", templateName)
	}

	// 2. Fetch week report.
	report, err := s.tsvc.GetWeekReport(ctx, profileName, weekDate)
	if err != nil {
		return domain.Template{}, fmt.Errorf("derive %q: %w", templateName, err)
	}

	// 3. Require at least one entry.
	if len(report.Entries) == 0 {
		return domain.Template{}, fmt.Errorf(
			"no entries in week of %s", report.WeekRef.StartDate.Format("2006-01-02"))
	}

	weekStart := report.WeekRef.StartDate

	// 4. Group entries by composite key.
	order := make([]rowKey, 0)
	groups := make(map[rowKey]*rowAccumulator)

	for _, e := range report.Entries {
		k := rowKey{
			kind:     e.Target.Kind,
			appID:    e.Target.AppID,
			itemID:   e.Target.ItemID,
			taskID:   e.Target.TaskID,
			typeID:   e.TimeType.ID,
			billable: e.Billable,
		}
		acc, exists := groups[k]
		if !exists {
			acc = &rowAccumulator{
				target:       e.Target,
				timeType:     e.TimeType,
				billable:     e.Billable,
				descriptions: make(map[string]int),
			}
			groups[k] = acc
			order = append(order, k)
		}
		acc.addEntry(e, weekStart)
	}

	// 5. Build rows slice in insertion order, then sort by total hours desc.
	// Skip rows with zero total hours (empty assignment slots from the week report).
	rows := make([]domain.TemplateRow, 0, len(order))
	for _, k := range order {
		acc := groups[k]
		if acc.hours.Total() == 0 {
			continue
		}
		desc := acc.mostCommonDescription()

		label := acc.target.DisplayName
		if label == "" {
			label = acc.target.DisplayRef
		}

		rows = append(rows, domain.TemplateRow{
			Label:       label,
			Target:      acc.target,
			TimeType:    acc.timeType,
			Billable:    acc.billable,
			Hours:       acc.hours,
			Description: desc,
			ResolverHints: domain.ResolverHints{
				TargetDisplayName: acc.target.DisplayName,
				TimeTypeName:      acc.timeType.Name,
			},
		})
	}

	// 6. Sort by total hours descending (stable for determinism).
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Hours.Total() > rows[j].Hours.Total()
	})

	// 7. Assign auto-generated row IDs.
	for i := range rows {
		rows[i].ID = fmt.Sprintf("row-%02d", i+1)
	}

	// 8. Build and save template.
	now := time.Now().UTC()
	tmpl := domain.Template{
		SchemaVersion: 1,
		Name:          templateName,
		CreatedAt:     now,
		ModifiedAt:    now,
		DerivedFrom: &domain.DeriveSource{
			Profile:   profileName,
			WeekStart: weekStart,
			DerivedAt: now,
		},
		Rows: rows,
	}

	if err := s.store.Save(tmpl); err != nil {
		return domain.Template{}, fmt.Errorf("derive %q: save: %w", templateName, err)
	}

	return tmpl, nil
}
