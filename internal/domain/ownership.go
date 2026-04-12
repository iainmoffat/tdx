package domain

// OwnershipChecker provides methods to check whether a time entry was created
// by tdx from a specific template row, and to embed or remove ownership
// markers in an entry description.
//
// Implementations may use a description-embedded marker (e.g. a comment
// suffix) or an external journal to track ownership.
type OwnershipChecker interface {
	// IsOwned reports whether entry was originally created by tdx for the
	// given template and row.
	IsOwned(entry TimeEntry, templateName, rowID string) bool

	// Mark returns a copy of description with an ownership marker appended
	// for the given template and row.
	Mark(description, templateName, rowID string) string

	// Unmark returns a copy of description with any ownership marker removed.
	Unmark(description string) string
}
