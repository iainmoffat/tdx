// Package week provides the `tdx time week ...` cobra command tree.
package week

import (
	"fmt"
	"strings"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// ParseDraftRef parses a "<YYYY-MM-DD>[/<name>]" token. Defaults name to "default".
// The date may be any day in the target week; the returned weekStart is the
// Sunday containing that date in EasternTZ.
func ParseDraftRef(s string) (time.Time, string, error) {
	if s == "" {
		return time.Time{}, "", fmt.Errorf("draft reference required")
	}
	var dateStr, name string
	if i := strings.IndexByte(s, '/'); i >= 0 {
		dateStr, name = s[:i], s[i+1:]
		if name == "" {
			return time.Time{}, "", fmt.Errorf("empty name after slash in %q", s)
		}
	} else {
		dateStr, name = s, "default"
	}
	d, err := time.ParseInLocation("2006-01-02", dateStr, domain.EasternTZ)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid date %q: %w", dateStr, err)
	}
	return domain.WeekRefContaining(d).StartDate, name, nil
}
