package domain

import (
	"fmt"
	"time"
)

// ApplyMode controls how an existing set of time entries is handled when a
// template is applied to a week.
type ApplyMode int

const (
	// ModeAdd appends template entries without removing or modifying existing ones.
	ModeAdd ApplyMode = 0
	// ModeReplaceMatching removes existing entries that match a template row
	// before creating new ones.
	ModeReplaceMatching ApplyMode = 1
	// ModeReplaceMine removes all entries owned by the current user before
	// creating new ones from the template.
	ModeReplaceMine ApplyMode = 2
)

// String returns the canonical string representation of the mode.
func (m ApplyMode) String() string {
	switch m {
	case ModeAdd:
		return "add"
	case ModeReplaceMatching:
		return "replace-matching"
	case ModeReplaceMine:
		return "replace-mine"
	}
	return fmt.Sprintf("ApplyMode(%d)", int(m))
}

// ParseApplyMode parses a string produced by ApplyMode.String() back into an
// ApplyMode. It returns an error for unrecognised values.
func ParseApplyMode(s string) (ApplyMode, error) {
	switch s {
	case "add":
		return ModeAdd, nil
	case "replace-matching":
		return ModeReplaceMatching, nil
	case "replace-mine":
		return ModeReplaceMine, nil
	}
	return 0, fmt.Errorf("unknown apply mode %q: must be add, replace-matching, or replace-mine", s)
}

// ActionKind classifies what the reconciliation engine will do for a single
// template row on a given day.
type ActionKind int

const (
	// ActionCreate means a new time entry will be created.
	ActionCreate ActionKind = 0
	// ActionUpdate means an existing time entry will be updated in place.
	ActionUpdate ActionKind = 1
	// ActionSkip means no change will be made (entry already matches or is
	// blocked by a non-fatal condition).
	ActionSkip ActionKind = 2
	// ActionDelete means an existing time entry will be deleted.
	ActionDelete ActionKind = 3
)

// String returns the canonical string representation of the action kind.
func (k ActionKind) String() string {
	switch k {
	case ActionCreate:
		return "create"
	case ActionUpdate:
		return "update"
	case ActionSkip:
		return "skip"
	case ActionDelete:
		return "delete"
	}
	return fmt.Sprintf("ActionKind(%d)", int(k))
}

// Action describes a single planned change produced by the reconciliation engine.
type Action struct {
	// Kind is what the engine will do.
	Kind ActionKind

	// RowID identifies the template row that triggered this action.
	RowID string

	// Date is the calendar day this action targets (midnight in EasternTZ).
	Date time.Time

	// Entry is the full EntryInput for ActionCreate actions.
	Entry EntryInput

	// ExistingID is the TD entry ID for ActionUpdate actions.
	ExistingID int

	// Patch is the set of fields to update for ActionUpdate actions.
	Patch EntryUpdate

	// DeleteEntryID is the TD entry ID for ActionDelete actions.
	DeleteEntryID int

	// SkipReason is a human-readable explanation for ActionSkip actions.
	SkipReason string
}

// BlockerKind classifies why the reconciliation engine cannot create or update
// an entry for a particular row/day combination.
type BlockerKind int

const (
	// BlockerLocked means the target day is administratively locked.
	BlockerLocked BlockerKind = 0
	// BlockerSubmitted means the week report has been submitted for approval.
	BlockerSubmitted BlockerKind = 1
	// BlockerApproved means the week report has already been approved.
	BlockerApproved BlockerKind = 2
	// BlockerTypeInvalid means the resolved time type is not valid for the target.
	BlockerTypeInvalid BlockerKind = 3
)

// String returns the canonical string representation of the blocker kind.
func (k BlockerKind) String() string {
	switch k {
	case BlockerLocked:
		return "locked"
	case BlockerSubmitted:
		return "submitted"
	case BlockerApproved:
		return "approved"
	case BlockerTypeInvalid:
		return "type-invalid"
	}
	return fmt.Sprintf("BlockerKind(%d)", int(k))
}

// Blocker describes a row/day pair that the reconciliation engine cannot
// process due to a hard constraint.
type Blocker struct {
	// Kind classifies the constraint.
	Kind BlockerKind

	// RowID identifies the template row affected.
	RowID string

	// Date is the calendar day affected (midnight in EasternTZ).
	Date time.Time

	// Reason is a human-readable explanation.
	Reason string
}

// ReconcileDiff is the complete planned change set produced by the
// reconciliation engine before any writes are performed.
type ReconcileDiff struct {
	// Actions is the ordered list of planned entry changes.
	Actions []Action

	// Blockers lists row/day pairs that cannot be written.
	Blockers []Blocker

	// DiffHash is a stable hash of the diff for idempotency checks.
	DiffHash string
}

// CountByKind tallies the actions by kind.
func (d ReconcileDiff) CountByKind() (creates, updates, skips int) {
	for _, a := range d.Actions {
		switch a.Kind {
		case ActionCreate:
			creates++
		case ActionUpdate:
			updates++
		case ActionSkip:
			skips++
		}
	}
	return
}

// CountByKindV2 tallies actions including deletes. The original CountByKind
// is kept for backwards-compatible call sites in tmplsvc; new draft-aware
// code uses V2.
func (d ReconcileDiff) CountByKindV2() (creates, updates, deletes, skips int) {
	for _, a := range d.Actions {
		switch a.Kind {
		case ActionCreate:
			creates++
		case ActionUpdate:
			updates++
		case ActionDelete:
			deletes++
		case ActionSkip:
			skips++
		}
	}
	return
}
