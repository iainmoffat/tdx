package tmplsvc

import "github.com/ipm/tdx/internal/domain"

// JournalChecker is a no-op implementation of domain.OwnershipChecker.
// Journal-based ownership tracking defers lookup to an append-only log file
// (~/.config/tdx/state/applies.log); for now this is a placeholder that never
// claims ownership. Journal writes happen in the Apply method, not here.
type JournalChecker struct{}

func (c *JournalChecker) IsOwned(_ domain.TimeEntry, _, _ string) bool { return false }
func (c *JournalChecker) Mark(description, _, _ string) string         { return description }
func (c *JournalChecker) Unmark(description string) string             { return description }
