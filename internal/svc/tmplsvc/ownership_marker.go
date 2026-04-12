package tmplsvc

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ipm/tdx/internal/domain"
)

var markerRe = regexp.MustCompile(`\[tdx:([^#\]]+)#([^\]]+)\]$`)

// MarkerChecker implements domain.OwnershipChecker via description markers.
// It appends a [tdx:<template-name>#<row-id>] suffix to the entry description
// so that ownership can be detected on subsequent reads without any external state.
type MarkerChecker struct{}

// IsOwned returns true when entry.Description ends with a marker that
// matches both templateName and rowID exactly.
func (c *MarkerChecker) IsOwned(entry domain.TimeEntry, templateName, rowID string) bool {
	m := markerRe.FindStringSubmatch(entry.Description)
	if m == nil {
		return false
	}
	return m[1] == templateName && m[2] == rowID
}

// Mark appends " [tdx:<templateName>#<rowID>]" to description.
// If description is empty the marker is returned alone (no leading space).
func (c *MarkerChecker) Mark(description, templateName, rowID string) string {
	marker := fmt.Sprintf("[tdx:%s#%s]", templateName, rowID)
	if description == "" {
		return marker
	}
	return description + " " + marker
}

// Unmark strips a trailing ownership marker (and its leading space) from
// description. If no marker is present the description is returned unchanged.
func (c *MarkerChecker) Unmark(description string) string {
	loc := markerRe.FindStringIndex(description)
	if loc == nil {
		return description
	}
	return strings.TrimRight(description[:loc[0]], " ")
}
