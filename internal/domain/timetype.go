package domain

import "strings"

// TimeType is a category of logged time, as TD exposes via /api/time/types.
// Phase 2 reads only; creating types is out of scope.
type TimeType struct {
	ID          int    `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Code        string `json:"code,omitempty" yaml:"code,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Billable    bool   `json:"billable" yaml:"billable"`
	Limited     bool   `json:"limited" yaml:"limited"`
	Active      bool   `json:"active" yaml:"active"`
}

// HasLimit is a small convenience wrapper so callers don't have to reach
// into the struct for a field that may grow additional semantics later.
func (t TimeType) HasLimit() bool { return t.Limited }

// FindTimeTypeByID returns the first time type with the given ID, if any.
func FindTimeTypeByID(types []TimeType, id int) (TimeType, bool) {
	for _, t := range types {
		if t.ID == id {
			return t, true
		}
	}
	return TimeType{}, false
}

// FindTimeTypeByName returns the first time type whose Name matches `name`
// case-insensitively. Used by `--type NAME` flag resolution.
func FindTimeTypeByName(types []TimeType, name string) (TimeType, bool) {
	lowered := strings.ToLower(strings.TrimSpace(name))
	for _, t := range types {
		if strings.ToLower(t.Name) == lowered {
			return t, true
		}
	}
	return TimeType{}, false
}
