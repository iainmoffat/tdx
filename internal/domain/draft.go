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
	RemoteStatus      ReportStatus   `yaml:"remoteStatus,omitempty" json:"remoteStatus,omitempty"`
	FromTemplate      string         `yaml:"fromTemplate,omitempty" json:"fromTemplate,omitempty"`
	FromDraft         string         `yaml:"fromDraft,omitempty" json:"fromDraft,omitempty"`
	ShiftedByDays     int            `yaml:"shiftedByDays,omitempty" json:"shiftedByDays,omitempty"`
}

// WeekDraft is the canonical local artifact for a single editable week.
type WeekDraft struct {
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

// PerCell holds per-cell metadata overrides (Phase C escape hatch). Empty
// pointer fields mean "use the row default."
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
