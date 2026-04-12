package domain

// BatchResult reports the outcome of a batch write operation.
type BatchResult struct {
	Succeeded []int          // IDs that succeeded
	Failed    []BatchFailure // entries that failed
}

// BatchFailure describes a single failed item in a batch operation.
type BatchFailure struct {
	ID      int
	Message string
}

// FullSuccess returns true when all items succeeded and at least one did.
func (r BatchResult) FullSuccess() bool {
	return len(r.Failed) == 0 && len(r.Succeeded) > 0
}

// PartialSuccess returns true when some items succeeded and some failed.
func (r BatchResult) PartialSuccess() bool {
	return len(r.Succeeded) > 0 && len(r.Failed) > 0
}

// TotalFailure returns true when no items succeeded and at least one failed.
func (r BatchResult) TotalFailure() bool {
	return len(r.Succeeded) == 0 && len(r.Failed) > 0
}
