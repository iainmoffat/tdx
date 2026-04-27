package draftsvc

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// reconcileDraft is the core engine: given a draft + at-pull-time cells +
// current remote report + locked days + watermark fingerprint, produce a
// ReconcileDiff describing what push will do.
//
// pulledByKey maps "rowID:weekday" (e.g. "row-01:Monday") to the DraftCell
// that was present at pull time. locked lists calendar days that cannot be
// written. pullWatermark is the remote fingerprint recorded at pull time;
// it is folded into the DiffHash so callers can detect stale diffs.
// userUID is embedded into EntryInput for any ActionCreate entries.
func reconcileDraft(
	draft domain.WeekDraft,
	pulledByKey map[string]domain.DraftCell,
	report domain.WeekReport,
	locked []domain.LockedDay,
	pullWatermark string,
	userUID string,
) (domain.ReconcileDiff, error) {
	lockedSet := map[string]bool{}
	for _, ld := range locked {
		lockedSet[ld.Date.Format("2006-01-02")] = true
	}

	remoteByID := map[int]domain.TimeEntry{}
	for _, e := range report.Entries {
		remoteByID[e.ID] = e
	}

	weekBlocked, weekBlockerKind, weekReason := false, domain.BlockerSubmitted, ""
	switch report.Status {
	case domain.ReportSubmitted:
		weekBlocked, weekReason = true, "week report is submitted for approval"
	case domain.ReportApproved:
		weekBlocked, weekBlockerKind, weekReason = true, domain.BlockerApproved, "week report is approved"
	}

	var actions []domain.Action
	var blockers []domain.Blocker

	weekRef := domain.WeekRefContaining(draft.WeekStart)

	for _, row := range draft.Rows {
		for _, cell := range row.Cells {
			date := weekRef.StartDate.AddDate(0, 0, int(cell.Day))
			dateStr := date.Format("2006-01-02")
			key := fmt.Sprintf("%s:%s", row.ID, cell.Day)
			pulled := pulledByKey[key]

			state := domain.ComputeCellState(pulled, cell)

			// Untouched pulled cells emit a noChange skip for visibility.
			if state == domain.CellUntouched {
				if cell.Hours > 0 && cell.SourceEntryID != 0 {
					actions = append(actions, domain.Action{
						Kind:       domain.ActionSkip,
						RowID:      row.ID,
						Date:       date,
						SkipReason: "noChange",
						ExistingID: cell.SourceEntryID,
					})
				}
				continue
			}

			// Week-level blockers apply before per-day checks.
			if weekBlocked {
				blockers = append(blockers, domain.Blocker{
					Kind:   weekBlockerKind,
					RowID:  row.ID,
					Date:   date,
					Reason: weekReason,
				})
				continue
			}

			// Per-day locked check.
			if lockedSet[dateStr] {
				blockers = append(blockers, domain.Blocker{
					Kind:   domain.BlockerLocked,
					RowID:  row.ID,
					Date:   date,
					Reason: fmt.Sprintf("%s is locked", dateStr),
				})
				continue
			}

			// Cleared pulled cell → delete.
			if cell.Hours == 0 && cell.SourceEntryID != 0 {
				actions = append(actions, domain.Action{
					Kind:          domain.ActionDelete,
					RowID:         row.ID,
					Date:          date,
					DeleteEntryID: cell.SourceEntryID,
				})
				continue
			}
			if cell.Hours == 0 {
				continue
			}

			rawMin := cell.Hours * 60
			mins := int(math.Round(rawMin))
			if math.Abs(rawMin-float64(mins)) > 0.001 {
				return domain.ReconcileDiff{}, fmt.Errorf(
					"row %s on %s: %.4fh produces non-integer minutes",
					row.ID, dateStr, cell.Hours)
			}

			entryInput := domain.EntryInput{
				UserUID:     userUID,
				Date:        date,
				Minutes:     mins,
				TimeTypeID:  row.TimeType.ID,
				Billable:    row.Billable,
				Target:      row.Target,
				Description: row.Description,
			}

			if cell.SourceEntryID != 0 {
				if existing, ok := remoteByID[cell.SourceEntryID]; ok {
					patch := buildDraftPatch(existing, entryInput)
					if patch.IsEmpty() {
						actions = append(actions, domain.Action{
							Kind:       domain.ActionSkip,
							RowID:      row.ID,
							Date:       date,
							ExistingID: cell.SourceEntryID,
							SkipReason: "noChange",
						})
					} else {
						actions = append(actions, domain.Action{
							Kind:          domain.ActionUpdate,
							RowID:         row.ID,
							Date:          date,
							ExistingID:    cell.SourceEntryID,
							BeforeMinutes: existing.Minutes,
							Patch:         patch,
						})
					}
					continue
				}
				// SourceEntryID set but entry no longer exists remotely.
				// Treat as stale reference and fall through to create.
			}

			actions = append(actions, domain.Action{
				Kind:  domain.ActionCreate,
				RowID: row.ID,
				Date:  date,
				Entry: entryInput,
			})
		}
	}

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

	hash := computeDraftDiffHash(actions, blockers, draft.Name, draft.WeekStart, pullWatermark)
	return domain.ReconcileDiff{Actions: actions, Blockers: blockers, DiffHash: hash}, nil
}

// buildDraftPatch computes the minimal EntryUpdate needed to bring existing
// in line with desired. Fields that already match are left nil (no-op).
func buildDraftPatch(existing domain.TimeEntry, desired domain.EntryInput) domain.EntryUpdate {
	var p domain.EntryUpdate
	if existing.Minutes != desired.Minutes {
		m := desired.Minutes
		p.Minutes = &m
	}
	if existing.TimeType.ID != desired.TimeTypeID {
		id := desired.TimeTypeID
		p.TimeTypeID = &id
	}
	if existing.Billable != desired.Billable {
		b := desired.Billable
		p.Billable = &b
	}
	if existing.Description != desired.Description {
		d := desired.Description
		p.Description = &d
	}
	return p
}

// computeDraftDiffHash produces a stable SHA-256 hex string over the full
// diff (actions + blockers) plus the draft name, week start, and the pull
// watermark. Changing any of these inputs changes the hash, so callers can
// use it to detect whether a previously-computed diff is still current.
func computeDraftDiffHash(
	actions []domain.Action,
	blockers []domain.Blocker,
	name string,
	weekStart time.Time,
	watermark string,
) string {
	type ha struct {
		Kind, RowID, Date, SkipReason                  string
		Minutes, TimeTypeID, ExistingID, DeleteEntryID int
		Billable                                       *bool
		Description                                    string
	}
	type hb struct{ Kind, RowID, Date string }
	type input struct {
		Actions                    []ha
		Blockers                   []hb
		Name, WeekStart, Watermark string
	}
	in := input{
		Name:      name,
		WeekStart: weekStart.Format("2006-01-02"),
		Watermark: watermark,
	}
	for _, a := range actions {
		x := ha{
			Kind:          a.Kind.String(),
			RowID:         a.RowID,
			Date:          a.Date.Format("2006-01-02"),
			SkipReason:    a.SkipReason,
			ExistingID:    a.ExistingID,
			DeleteEntryID: a.DeleteEntryID,
		}
		switch a.Kind {
		case domain.ActionCreate:
			x.Minutes = a.Entry.Minutes
			x.TimeTypeID = a.Entry.TimeTypeID
			b := a.Entry.Billable
			x.Billable = &b
			x.Description = a.Entry.Description
		case domain.ActionUpdate:
			if a.Patch.Minutes != nil {
				x.Minutes = *a.Patch.Minutes
			}
			if a.Patch.TimeTypeID != nil {
				x.TimeTypeID = *a.Patch.TimeTypeID
			}
			x.Billable = a.Patch.Billable
			if a.Patch.Description != nil {
				x.Description = *a.Patch.Description
			}
		}
		in.Actions = append(in.Actions, x)
	}
	for _, bl := range blockers {
		in.Blockers = append(in.Blockers, hb{
			Kind:  bl.Kind.String(),
			RowID: bl.RowID,
			Date:  bl.Date.Format("2006-01-02"),
		})
	}
	data, _ := json.Marshal(in)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
