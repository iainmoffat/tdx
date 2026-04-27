package domain

import (
	"fmt"
	"time"
)

// ProvenanceKind enumerates how a draft came into existence.
type ProvenanceKind string

const (
	ProvenanceBlank        ProvenanceKind = "blank"
	ProvenancePulled       ProvenanceKind = "pulled"
	ProvenanceFromTemplate ProvenanceKind = "from-template"
	ProvenanceFromDraft    ProvenanceKind = "from-draft"
)

// DraftProvenance records how a draft was created.
type DraftProvenance struct {
	Kind              ProvenanceKind `yaml:"kind" json:"kind"`
	PulledAt          time.Time      `yaml:"pulledAt,omitempty" json:"pulledAt,omitempty"`
	RemoteFingerprint string         `yaml:"remoteFingerprint,omitempty" json:"remoteFingerprint,omitempty"`
	RemoteStatus      ReportStatus   `yaml:"remoteStatus,omitempty" json:"remoteStatus,omitempty"` // open|submitted|approved|rejected
	FromTemplate      string         `yaml:"fromTemplate,omitempty" json:"fromTemplate,omitempty"`
	FromDraft         string         `yaml:"fromDraft,omitempty" json:"fromDraft,omitempty"`
	ShiftedByDays     int            `yaml:"shiftedByDays,omitempty" json:"shiftedByDays,omitempty"`
}

// WeekDraft is the canonical local artifact for a single editable week.
type WeekDraft struct {
	// SchemaVersion identifies the draft on-disk schema. Bumped when a
	// breaking change is made; load-time migration handles older versions.
	// Phase A initial release: 1.
	SchemaVersion int             `yaml:"schemaVersion" json:"schemaVersion"`
	Profile       string          `yaml:"profile" json:"profile"`
	WeekStart     time.Time       `yaml:"weekStart" json:"weekStart"`
	Name          string          `yaml:"name" json:"name"`
	Notes         string          `yaml:"notes,omitempty" json:"notes,omitempty"`
	Tags          []string        `yaml:"tags,omitempty" json:"tags,omitempty"`
	Provenance    DraftProvenance `yaml:"provenance" json:"provenance"`
	CreatedAt     time.Time       `yaml:"createdAt" json:"createdAt"`
	ModifiedAt    time.Time       `yaml:"modifiedAt" json:"modifiedAt"`
	PushedAt      *time.Time      `yaml:"pushedAt,omitempty" json:"pushedAt,omitempty"`
	Rows          []DraftRow      `yaml:"rows" json:"rows"`
	// Archived hides the draft from default `list` output. Soft-archive: no
	// file motion, fully reversible via `unarchive`.
	Archived bool `yaml:"archived,omitempty" json:"archived,omitempty"`
}

// DraftRow is one row of a draft (target + type + billable + cells).
type DraftRow struct {
	ID            string        `yaml:"id" json:"id"`
	Label         string        `yaml:"label,omitempty" json:"label,omitempty"`
	Target        Target        `yaml:"target" json:"target"`
	TimeType      TimeType      `yaml:"timeType" json:"timeType"`
	Description   string        `yaml:"description,omitempty" json:"description,omitempty"`
	Billable      bool          `yaml:"billable" json:"billable"`
	ResolverHints ResolverHints `yaml:"resolverHints,omitempty" json:"resolverHints,omitempty"`
	Cells         []DraftCell   `yaml:"cells" json:"cells"`
}

// DraftCell is the atomic unit of a draft: one row × one weekday.
type DraftCell struct {
	Day           time.Weekday `yaml:"day" json:"day"`
	Hours         float64      `yaml:"hours" json:"hours"`
	SourceEntryID int          `yaml:"sourceEntryID,omitempty" json:"sourceEntryID,omitempty"`
	PerCell       *PerCell     `yaml:"perCell,omitempty" json:"perCell,omitempty"`
}

// PerCell holds per-cell metadata overrides (Phase C escape hatch).
// All pointer fields are optional: nil means "inherit from the row";
// a non-nil value overrides the row default for that one cell.
type PerCell struct {
	Description *string `yaml:"description,omitempty" json:"description,omitempty"`
	TimeTypeID  *int    `yaml:"timeTypeID,omitempty" json:"timeTypeID,omitempty"`
	Billable    *bool   `yaml:"billable,omitempty" json:"billable,omitempty"`
}

// Validate checks structural integrity of the draft.
func (d WeekDraft) Validate() error {
	if d.Profile == "" {
		return fmt.Errorf("draft profile is required")
	}
	if d.Name == "" {
		return fmt.Errorf("draft name is required")
	}
	if d.WeekStart.IsZero() {
		return fmt.Errorf("draft weekStart is required")
	}
	if d.WeekStart.In(EasternTZ).Weekday() != time.Sunday {
		return fmt.Errorf("draft weekStart must be a Sunday in EasternTZ (got %s)",
			d.WeekStart.Weekday())
	}
	// Empty Rows is intentionally permitted: a nascent draft (created via
	// `tdx time week new` with no template seed) has zero rows until the
	// user adds them.
	seen := make(map[string]struct{}, len(d.Rows))
	for i, row := range d.Rows {
		if row.ID == "" {
			return fmt.Errorf("draft row[%d] has empty ID", i)
		}
		if _, dup := seen[row.ID]; dup {
			return fmt.Errorf("draft has duplicate row ID %q", row.ID)
		}
		seen[row.ID] = struct{}{}
	}
	return nil
}

// CellState classifies a draft cell relative to its pulled origin.
type CellState string

const (
	CellUntouched CellState = "untouched"
	CellEdited    CellState = "edited"
	CellAdded     CellState = "added"
	CellConflict  CellState = "conflict"
	CellInvalid   CellState = "invalid"
)

// SyncState classifies a draft as a whole.
type SyncState string

const (
	SyncClean      SyncState = "clean"
	SyncDirty      SyncState = "dirty"
	SyncConflicted SyncState = "conflicted"
)

// DraftSyncState bundles the sync verdict + drift flag + cell counts.
type DraftSyncState struct {
	Sync       SyncState
	Stale      bool
	Untouched  int
	Edited     int
	Added      int
	Conflict   int
	TotalHours float64
}

// ComputeCellState classifies the current cell against the cell at pull time.
// A zero-valued pulledAtPullTime means "no entry was pulled here" (added cell).
func ComputeCellState(pulledAtPullTime, current DraftCell) CellState {
	if pulledAtPullTime.SourceEntryID == 0 && current.SourceEntryID == 0 {
		if current.Hours > 0 {
			return CellAdded
		}
		return CellUntouched
	}
	if pulledAtPullTime.Hours == current.Hours {
		return CellUntouched
	}
	return CellEdited
}

// ComputeSyncState walks the draft's cells, comparing each to its pulled
// counterpart (keyed by "rowID:weekday"), and returns the aggregate state.
//
// pulledFingerprint should be the remote fingerprint observed at the most
// recent successful pull or push; if it differs from the draft's stored
// remoteFingerprint, the Stale flag is set.
//
// Key format: "rowID:weekday" where weekday is time.Weekday.String() (e.g.
// "Monday", "Tuesday"). Callers building the pulledByKey map must use the
// same case-sensitive format.
func ComputeSyncState(draft WeekDraft, pulledByKey map[string]DraftCell, currentRemoteFingerprint string) DraftSyncState {
	s := DraftSyncState{Sync: SyncClean}
	for _, row := range draft.Rows {
		for _, cell := range row.Cells {
			key := fmt.Sprintf("%s:%s", row.ID, cell.Day)
			pulled := pulledByKey[key]
			switch ComputeCellState(pulled, cell) {
			case CellUntouched:
				s.Untouched++
			case CellEdited:
				s.Edited++
				if s.Sync == SyncClean {
					s.Sync = SyncDirty
				}
			case CellAdded:
				s.Added++
				if s.Sync == SyncClean {
					s.Sync = SyncDirty
				}
			case CellConflict:
				s.Conflict++
				s.Sync = SyncConflicted
			}
			s.TotalHours += cell.Hours
		}
	}
	if currentRemoteFingerprint != "" && draft.Provenance.RemoteFingerprint != "" {
		s.Stale = currentRemoteFingerprint != draft.Provenance.RemoteFingerprint
	}
	return s
}
