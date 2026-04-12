package tmplsvc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/ipm/tdx/internal/domain"
)

// ReconcileInput holds the parameters for a reconciliation run.
type ReconcileInput struct {
	// Template is the template to project onto the week.
	Template domain.Template

	// WeekRef identifies the target Sun–Sat week.
	WeekRef domain.WeekRef

	// Mode controls how existing entries are handled.
	Mode domain.ApplyMode

	// DaysFilter restricts reconciliation to specific weekdays. An empty
	// slice means all days are eligible.
	DaysFilter []time.Weekday

	// Overrides replaces individual row/day hour values before projection.
	Overrides []Override

	// Round, when true, allows non-integer minute values to be rounded to
	// the nearest whole minute instead of producing an error.
	Round bool

	// UserUID is the TD user UID to populate in EntryInput for Create
	// actions. The reconciler does not look this up itself.
	UserUID string

	// Checker is used by ModeReplaceMine to test entry ownership. May be
	// nil for other modes.
	Checker domain.OwnershipChecker
}

// Override replaces the template hours for a specific row and weekday.
type Override struct {
	RowID string
	Day   time.Weekday
	Hours float64
}

// matchKey is the composite identity used to determine whether a template row
// already has a corresponding entry on a given day. Two entries with the same
// matchKey on the same calendar date are considered duplicates.
type matchKey struct {
	kind       domain.TargetKind
	appID      int
	itemID     int
	taskID     int
	timeTypeID int
	dateStr    string // "YYYY-MM-DD" — DST-safe comparison
}

// Reconcile projects a template onto a week and classifies the required
// actions. It fetches the current week report and locked days, then for each
// template row and day it determines whether to create, update, skip, or block.
//
// The returned ReconcileDiff is a preview — no writes are performed.
func (s *Service) Reconcile(ctx context.Context, profileName string, input ReconcileInput) (domain.ReconcileDiff, error) {
	// 1. Fetch current state.
	report, err := s.tsvc.GetWeekReport(ctx, profileName, input.WeekRef.StartDate)
	if err != nil {
		return domain.ReconcileDiff{}, fmt.Errorf("reconcile: get week report: %w", err)
	}

	lockedDays, err := s.tsvc.GetLockedDays(ctx, profileName, input.WeekRef.StartDate, input.WeekRef.EndDate)
	if err != nil {
		return domain.ReconcileDiff{}, fmt.Errorf("reconcile: get locked days: %w", err)
	}

	// 2. Build locked-day set keyed by "YYYY-MM-DD".
	lockedSet := make(map[string]bool, len(lockedDays))
	for _, ld := range lockedDays {
		lockedSet[ld.Date.Format("2006-01-02")] = true
	}

	// 3. Build the days-filter set for O(1) lookups.
	filterSet := make(map[time.Weekday]bool, len(input.DaysFilter))
	for _, d := range input.DaysFilter {
		filterSet[d] = true
	}
	hasFilter := len(filterSet) > 0

	// 4. Build override lookup: (rowID, weekday) → hours.
	overrideMap := make(map[string]float64, len(input.Overrides))
	for _, o := range input.Overrides {
		key := fmt.Sprintf("%s:%d", o.RowID, o.Day)
		overrideMap[key] = o.Hours
	}

	// 5. Index existing entries by match key for fast lookup.
	existingByKey := make(map[matchKey]domain.TimeEntry, len(report.Entries))
	for _, e := range report.Entries {
		mk := matchKey{
			kind:       e.Target.Kind,
			appID:      e.Target.AppID,
			itemID:     e.Target.ItemID,
			taskID:     e.Target.TaskID,
			timeTypeID: e.TimeType.ID,
			dateStr:    e.Date.Format("2006-01-02"),
		}
		existingByKey[mk] = e
	}

	// 6. Determine week-level blockers.
	weekBlocked := false
	var weekBlockerKind domain.BlockerKind
	var weekBlockerReason string
	switch report.Status {
	case domain.ReportSubmitted:
		weekBlocked = true
		weekBlockerKind = domain.BlockerSubmitted
		weekBlockerReason = "week report is submitted for approval"
	case domain.ReportApproved:
		weekBlocked = true
		weekBlockerKind = domain.BlockerApproved
		weekBlockerReason = "week report is approved"
	}

	// 7. Iterate rows × days.
	var actions []domain.Action
	var blockers []domain.Blocker

	for _, row := range input.Template.Rows {
		for dayOffset := 0; dayOffset < 7; dayOffset++ {
			date := input.WeekRef.StartDate.AddDate(0, 0, dayOffset)
			weekday := date.Weekday()

			// Resolve hours: check override first, then template.
			overrideKey := fmt.Sprintf("%s:%d", row.ID, weekday)
			hours := row.Hours.ForDay(weekday)
			if ov, ok := overrideMap[overrideKey]; ok {
				hours = ov
			}

			// Skip if zero hours for this day.
			if hours == 0 {
				continue
			}

			// Skip if day is filtered out.
			if hasFilter && !filterSet[weekday] {
				continue
			}

			// Convert hours to minutes.
			rawMinutes := hours * 60
			rounded := math.Round(rawMinutes)
			if !input.Round && math.Abs(rawMinutes-rounded) > 0.001 {
				return domain.ReconcileDiff{}, fmt.Errorf(
					"reconcile: row %q on %s: %.4f hours produces non-integer minutes (%.4f); use --round to allow rounding",
					row.ID, weekday, hours, rawMinutes)
			}
			minutes := int(rounded)

			dateStr := date.Format("2006-01-02")

			// Check week-level blocker.
			if weekBlocked {
				blockers = append(blockers, domain.Blocker{
					Kind:   weekBlockerKind,
					RowID:  row.ID,
					Date:   date,
					Reason: weekBlockerReason,
				})
				continue
			}

			// Check locked day.
			if lockedSet[dateStr] {
				blockers = append(blockers, domain.Blocker{
					Kind:   domain.BlockerLocked,
					RowID:  row.ID,
					Date:   date,
					Reason: fmt.Sprintf("%s is locked", dateStr),
				})
				continue
			}

			// Look for existing matching entry.
			mk := matchKey{
				kind:       row.Target.Kind,
				appID:      row.Target.AppID,
				itemID:     row.Target.ItemID,
				taskID:     row.Target.TaskID,
				timeTypeID: row.TimeType.ID,
				dateStr:    dateStr,
			}

			existing, found := existingByKey[mk]

			// Build the EntryInput used for creates.
			entry := domain.EntryInput{
				UserUID:     input.UserUID,
				Date:        date,
				Minutes:     minutes,
				TimeTypeID:  row.TimeType.ID,
				Billable:    row.Billable,
				Target:      row.Target,
				Description: row.Description,
			}

			switch input.Mode {
			case domain.ModeAdd:
				if found {
					actions = append(actions, domain.Action{
						Kind:       domain.ActionSkip,
						RowID:      row.ID,
						Date:       date,
						SkipReason: "alreadyExists",
					})
				} else {
					actions = append(actions, domain.Action{
						Kind:  domain.ActionCreate,
						RowID: row.ID,
						Date:  date,
						Entry: entry,
					})
				}

			case domain.ModeReplaceMatching:
				if found {
					patch := buildPatch(existing, entry)
					if patch.IsEmpty() {
						actions = append(actions, domain.Action{
							Kind:       domain.ActionSkip,
							RowID:      row.ID,
							Date:       date,
							ExistingID: existing.ID,
							SkipReason: "alreadyMatches",
						})
					} else {
						actions = append(actions, domain.Action{
							Kind:       domain.ActionUpdate,
							RowID:      row.ID,
							Date:       date,
							ExistingID: existing.ID,
							Patch:      patch,
						})
					}
				} else {
					actions = append(actions, domain.Action{
						Kind:  domain.ActionCreate,
						RowID: row.ID,
						Date:  date,
						Entry: entry,
					})
				}

			case domain.ModeReplaceMine:
				if found {
					if input.Checker == nil {
						return domain.ReconcileDiff{}, fmt.Errorf("reconcile: replace-mine mode requires an ownership checker")
					}
					if input.Checker.IsOwned(existing, input.Template.Name, row.ID) {
						patch := buildPatch(existing, entry)
						if patch.IsEmpty() {
							actions = append(actions, domain.Action{
								Kind:       domain.ActionSkip,
								RowID:      row.ID,
								Date:       date,
								ExistingID: existing.ID,
								SkipReason: "alreadyMatches",
							})
						} else {
							actions = append(actions, domain.Action{
								Kind:       domain.ActionUpdate,
								RowID:      row.ID,
								Date:       date,
								ExistingID: existing.ID,
								Patch:      patch,
							})
						}
					} else {
						actions = append(actions, domain.Action{
							Kind:       domain.ActionSkip,
							RowID:      row.ID,
							Date:       date,
							SkipReason: "notOwnedByTemplate",
						})
					}
				} else {
					actions = append(actions, domain.Action{
						Kind:  domain.ActionCreate,
						RowID: row.ID,
						Date:  date,
						Entry: entry,
					})
				}
			}
		}
	}

	// 8. Sort actions and blockers by (RowID, Date) for stable hashing.
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].RowID != actions[j].RowID {
			return actions[i].RowID < actions[j].RowID
		}
		return actions[i].Date.Before(actions[j].Date)
	})
	sort.SliceStable(blockers, func(i, j int) bool {
		if blockers[i].RowID != blockers[j].RowID {
			return blockers[i].RowID < blockers[j].RowID
		}
		return blockers[i].Date.Before(blockers[j].Date)
	})

	// 9. Compute DiffHash.
	diffHash, err := computeDiffHash(actions, blockers, input.Mode, input.Template.Name, input.WeekRef.StartDate)
	if err != nil {
		return domain.ReconcileDiff{}, fmt.Errorf("reconcile: compute diff hash: %w", err)
	}

	return domain.ReconcileDiff{
		Actions:  actions,
		Blockers: blockers,
		DiffHash: diffHash,
	}, nil
}

// buildPatch compares an existing entry to the desired state and returns an
// EntryUpdate with only the changed fields set.
func buildPatch(existing domain.TimeEntry, desired domain.EntryInput) domain.EntryUpdate {
	var patch domain.EntryUpdate
	if existing.Minutes != desired.Minutes {
		m := desired.Minutes
		patch.Minutes = &m
	}
	if existing.TimeType.ID != desired.TimeTypeID {
		id := desired.TimeTypeID
		patch.TimeTypeID = &id
	}
	if existing.Billable != desired.Billable {
		b := desired.Billable
		patch.Billable = &b
	}
	if existing.Description != desired.Description {
		d := desired.Description
		patch.Description = &d
	}
	return patch
}

// diffHashInput is the canonical structure hashed to produce DiffHash.
type diffHashInput struct {
	Actions      []diffHashAction  `json:"actions"`
	Blockers     []diffHashBlocker `json:"blockers"`
	Mode         string            `json:"mode"`
	TemplateName string            `json:"templateName"`
	WeekStart    string            `json:"weekStart"`
}

type diffHashAction struct {
	Kind        string `json:"kind"`
	RowID       string `json:"rowID"`
	Date        string `json:"date"`
	Minutes     int    `json:"minutes,omitempty"`
	TimeTypeID  int    `json:"timeTypeID,omitempty"`
	Billable    *bool  `json:"billable,omitempty"`
	Description string `json:"description,omitempty"`
	ExistingID  int    `json:"existingID,omitempty"`
	SkipReason  string `json:"skipReason,omitempty"`
}

type diffHashBlocker struct {
	Kind  string `json:"kind"`
	RowID string `json:"rowID"`
	Date  string `json:"date"`
}

func computeDiffHash(actions []domain.Action, blockers []domain.Blocker, mode domain.ApplyMode, tmplName string, weekStart time.Time) (string, error) {
	input := diffHashInput{
		Mode:         mode.String(),
		TemplateName: tmplName,
		WeekStart:    weekStart.Format("2006-01-02"),
	}

	input.Actions = make([]diffHashAction, len(actions))
	for i, a := range actions {
		ha := diffHashAction{
			Kind:       a.Kind.String(),
			RowID:      a.RowID,
			Date:       a.Date.Format("2006-01-02"),
			SkipReason: a.SkipReason,
			ExistingID: a.ExistingID,
		}
		switch a.Kind {
		case domain.ActionCreate:
			ha.Minutes = a.Entry.Minutes
			ha.TimeTypeID = a.Entry.TimeTypeID
			b := a.Entry.Billable
			ha.Billable = &b
			ha.Description = a.Entry.Description
		case domain.ActionUpdate:
			if a.Patch.Minutes != nil {
				ha.Minutes = *a.Patch.Minutes
			}
			if a.Patch.TimeTypeID != nil {
				ha.TimeTypeID = *a.Patch.TimeTypeID
			}
			ha.Billable = a.Patch.Billable
			if a.Patch.Description != nil {
				ha.Description = *a.Patch.Description
			}
		}
		input.Actions[i] = ha
	}

	input.Blockers = make([]diffHashBlocker, len(blockers))
	for i, b := range blockers {
		input.Blockers[i] = diffHashBlocker{
			Kind:  b.Kind.String(),
			RowID: b.RowID,
			Date:  b.Date.Format("2006-01-02"),
		}
	}

	data, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}
