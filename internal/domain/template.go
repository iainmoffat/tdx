package domain

import (
	"fmt"
	"math"
	"time"
)

// DeriveSource controls how a template row's time type is resolved when
// applying the template to a week.
type DeriveSource string

const (
	// DeriveFixed uses the TimeTypeID set explicitly on the row.
	DeriveFixed DeriveSource = "fixed"
	// DeriveTarget resolves the time type from the target's component lookup.
	DeriveTarget DeriveSource = "target"
)

// ResolverHints carries optional metadata that aids time-type resolution when
// DeriveSource is DeriveTarget.
type ResolverHints struct {
	PreferBillable bool `json:"preferBillable,omitempty" yaml:"preferBillable,omitempty"`
}

// WeekHours holds per-weekday hour allocations for a template row.
// Fields are omitted from serialised output when zero.
type WeekHours struct {
	Sun float64 `json:"sun,omitempty" yaml:"sun,omitempty"`
	Mon float64 `json:"mon,omitempty" yaml:"mon,omitempty"`
	Tue float64 `json:"tue,omitempty" yaml:"tue,omitempty"`
	Wed float64 `json:"wed,omitempty" yaml:"wed,omitempty"`
	Thu float64 `json:"thu,omitempty" yaml:"thu,omitempty"`
	Fri float64 `json:"fri,omitempty" yaml:"fri,omitempty"`
	Sat float64 `json:"sat,omitempty" yaml:"sat,omitempty"`
}

// Total returns the sum of all seven day allocations.
func (wh WeekHours) Total() float64 {
	return wh.Sun + wh.Mon + wh.Tue + wh.Wed + wh.Thu + wh.Fri + wh.Sat
}

// ForDay returns the hour allocation for the given weekday.
// It returns 0 for any unrecognised weekday value.
func (wh WeekHours) ForDay(d time.Weekday) float64 {
	switch d {
	case time.Sunday:
		return wh.Sun
	case time.Monday:
		return wh.Mon
	case time.Tuesday:
		return wh.Tue
	case time.Wednesday:
		return wh.Wed
	case time.Thursday:
		return wh.Thu
	case time.Friday:
		return wh.Fri
	case time.Saturday:
		return wh.Sat
	}
	return 0
}

// SetDay sets the hour allocation for the given weekday.
// Unrecognised weekday values are silently ignored.
func (wh *WeekHours) SetDay(d time.Weekday, hours float64) {
	switch d {
	case time.Sunday:
		wh.Sun = hours
	case time.Monday:
		wh.Mon = hours
	case time.Tuesday:
		wh.Tue = hours
	case time.Wednesday:
		wh.Wed = hours
	case time.Thursday:
		wh.Thu = hours
	case time.Friday:
		wh.Fri = hours
	case time.Saturday:
		wh.Sat = hours
	}
}

// ToMinutesExact converts the hour allocation for d to whole minutes.
// It returns (minutes, true) only when the product is an exact integer
// (no fractional minute). Returns (0, false) for non-exact values.
func (wh WeekHours) ToMinutesExact(d time.Weekday) (int, bool) {
	h := wh.ForDay(d)
	raw := h * 60
	rounded := math.Round(raw)
	if math.Abs(raw-rounded) > 1e-9 {
		return 0, false
	}
	return int(rounded), true
}

// TemplateRow describes a single row in a schedule template — one line of
// work against a specific target.
type TemplateRow struct {
	// ID is a short, unique key within the template (e.g. "standup", "tickets").
	ID string `json:"id" yaml:"id"`

	// Description is the optional default time-entry description.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Target is the TD work item this row logs time against.
	Target Target `json:"target" yaml:"target"`

	// TimeTypeID is the explicit time type when Derive is DeriveFixed.
	TimeTypeID int `json:"timeTypeID,omitempty" yaml:"timeTypeID,omitempty"`

	// Derive controls how the time type is resolved at apply time.
	Derive DeriveSource `json:"derive,omitempty" yaml:"derive,omitempty"`

	// Hints provides additional hints for DeriveTarget resolution.
	Hints ResolverHints `json:"hints,omitempty" yaml:"hints,omitempty"`

	// Hours holds the per-weekday hour allocations for this row.
	Hours WeekHours `json:"hours,omitempty" yaml:"hours,omitempty"`
}

// Template is a named schedule template composed of one or more rows.
type Template struct {
	// Name is the human-readable identifier for this template.
	Name string `json:"name" yaml:"name"`

	// Description is an optional longer description of the template's purpose.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Rows is the ordered list of work rows in this template.
	Rows []TemplateRow `json:"rows" yaml:"rows"`
}

// Validate returns nil if the template is structurally valid.
// It checks: non-empty name, at least one row, no empty row IDs, no
// duplicate row IDs.
func (t Template) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if len(t.Rows) == 0 {
		return fmt.Errorf("template %q must have at least one row", t.Name)
	}
	seen := make(map[string]struct{}, len(t.Rows))
	for i, row := range t.Rows {
		if row.ID == "" {
			return fmt.Errorf("template %q row[%d] has an empty ID", t.Name, i)
		}
		if _, dup := seen[row.ID]; dup {
			return fmt.Errorf("template %q has duplicate row ID %q", t.Name, row.ID)
		}
		seen[row.ID] = struct{}{}
	}
	return nil
}
