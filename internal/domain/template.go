package domain

import (
	"fmt"
	"math"
	"time"
)

// Template is a reusable weekly time pattern.
type Template struct {
	SchemaVersion int            `yaml:"schemaVersion" json:"schemaVersion"`
	Name          string         `yaml:"name" json:"name"`
	Description   string         `yaml:"description,omitempty" json:"description,omitempty"`
	Tags          []string       `yaml:"tags,omitempty" json:"tags,omitempty"`
	CreatedAt     time.Time      `yaml:"createdAt" json:"createdAt"`
	ModifiedAt    time.Time      `yaml:"modifiedAt" json:"modifiedAt"`
	DerivedFrom   *DeriveSource  `yaml:"derivedFrom,omitempty" json:"derivedFrom,omitempty"`
	Rows          []TemplateRow  `yaml:"rows" json:"rows"`
}

// DeriveSource records which live week a template was derived from.
type DeriveSource struct {
	Profile   string    `yaml:"profile" json:"profile"`
	WeekStart time.Time `yaml:"weekStart" json:"weekStart"`
	DerivedAt time.Time `yaml:"derivedAt" json:"derivedAt"`
}

// TemplateRow is one row in a template — a target + type + hours per day.
type TemplateRow struct {
	ID            string        `yaml:"id" json:"id"`
	Label         string        `yaml:"label,omitempty" json:"label,omitempty"`
	Target        Target        `yaml:"target" json:"target"`
	TimeType      TimeType      `yaml:"timeType" json:"timeType"`
	Description   string        `yaml:"description,omitempty" json:"description,omitempty"`
	Billable      bool          `yaml:"billable" json:"billable"`
	Hours         WeekHours     `yaml:"hours" json:"hours"`
	ResolverHints ResolverHints `yaml:"resolverHints,omitempty" json:"resolverHints,omitempty"`
}

// ResolverHints stores display names captured at derive time for
// re-validation and drift detection at apply time.
type ResolverHints struct {
	TargetDisplayName string `yaml:"targetDisplayName,omitempty" json:"targetDisplayName,omitempty"`
	TargetAppName     string `yaml:"targetAppName,omitempty" json:"targetAppName,omitempty"`
	TimeTypeName      string `yaml:"timeTypeName,omitempty" json:"timeTypeName,omitempty"`
}

// WeekHours holds per-weekday hour allocations for a template row.
// Fields map to time.Weekday: Sun=0 through Sat=6. Zero values mean
// "no entry for this day."
type WeekHours struct {
	Sun float64 `yaml:"sun,omitempty" json:"sun,omitempty"`
	Mon float64 `yaml:"mon,omitempty" json:"mon,omitempty"`
	Tue float64 `yaml:"tue,omitempty" json:"tue,omitempty"`
	Wed float64 `yaml:"wed,omitempty" json:"wed,omitempty"`
	Thu float64 `yaml:"thu,omitempty" json:"thu,omitempty"`
	Fri float64 `yaml:"fri,omitempty" json:"fri,omitempty"`
	Sat float64 `yaml:"sat,omitempty" json:"sat,omitempty"`
}

// Total returns the sum of all seven day allocations.
func (h WeekHours) Total() float64 {
	return h.Sun + h.Mon + h.Tue + h.Wed + h.Thu + h.Fri + h.Sat
}

// ForDay returns the hour allocation for the given weekday.
func (h WeekHours) ForDay(d time.Weekday) float64 {
	switch d {
	case time.Sunday:
		return h.Sun
	case time.Monday:
		return h.Mon
	case time.Tuesday:
		return h.Tue
	case time.Wednesday:
		return h.Wed
	case time.Thursday:
		return h.Thu
	case time.Friday:
		return h.Fri
	case time.Saturday:
		return h.Sat
	}
	return 0
}

// SetDay sets the hour allocation for the given weekday.
func (h *WeekHours) SetDay(d time.Weekday, hours float64) {
	switch d {
	case time.Sunday:
		h.Sun = hours
	case time.Monday:
		h.Mon = hours
	case time.Tuesday:
		h.Tue = hours
	case time.Wednesday:
		h.Wed = hours
	case time.Thursday:
		h.Thu = hours
	case time.Friday:
		h.Fri = hours
	case time.Saturday:
		h.Sat = hours
	}
}

// ToMinutesExact converts the hour allocation for d to whole minutes.
// Returns (minutes, true) only when the product is an exact integer.
func (h WeekHours) ToMinutesExact(d time.Weekday) (int, bool) {
	hours := h.ForDay(d)
	raw := hours * 60
	rounded := math.Round(raw)
	if math.Abs(raw-rounded) > 0.001 {
		return 0, false
	}
	return int(rounded), true
}

// Validate checks structural integrity of a template.
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
