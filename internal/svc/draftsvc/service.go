package draftsvc

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/timesvc"
)

// timeWriter is the subset of timesvc.Service that draftsvc consumes.
// timesvc.Service satisfies this interface implicitly. Tests can supply
// a mock to avoid hitting a live tenant.
type timeWriter interface {
	AddEntry(ctx context.Context, profile string, input domain.EntryInput) (domain.TimeEntry, error)
	UpdateEntry(ctx context.Context, profile string, id int, update domain.EntryUpdate) (domain.TimeEntry, error)
	DeleteEntry(ctx context.Context, profile string, id int) error
	GetWeekReport(ctx context.Context, profile string, date time.Time) (domain.WeekReport, error)
	GetLockedDays(ctx context.Context, profile string, from, to time.Time) ([]domain.LockedDay, error)
	TimeTypesForTarget(ctx context.Context, profile string, target domain.Target) ([]domain.TimeType, error)
}

// Service is the draft-aware service layer.
type Service struct {
	paths     config.Paths
	store     *Store
	snapshots *SnapshotStore
	tsvc      timeWriter
}

// NewService constructs a Service backed by paths and the live TD time service.
func NewService(paths config.Paths, tsvc *timesvc.Service) *Service {
	return newServiceWithTimeWriter(paths, tsvc)
}

// newServiceWithTimeWriter constructs a Service using any timeWriter implementation.
// Used in tests to inject a mock without hitting a live tenant.
func newServiceWithTimeWriter(paths config.Paths, tsvc timeWriter) *Service {
	return &Service{
		paths:     paths,
		store:     NewStore(paths),
		snapshots: NewSnapshotStore(paths, 10),
		tsvc:      tsvc,
	}
}

// Store returns the underlying draft store.
func (s *Service) Store() *Store { return s.store }

// Snapshots returns the underlying snapshot store.
func (s *Service) Snapshots() *SnapshotStore { return s.snapshots }

// Pull fetches the live week and saves it as a draft. Refuses to overwrite
// a dirty draft unless force=true (auto-snapshots first when forcing).
func (s *Service) Pull(ctx context.Context, profile string, weekStart time.Time, name string, force bool) (domain.WeekDraft, error) {
	if name == "" {
		name = "default"
	}
	if existing, err := s.store.Load(profile, weekStart, name); err == nil {
		pulled, _ := s.PulledCellsByKey(profile, weekStart, name)
		sync := domain.ComputeSyncState(existing, pulled, "")
		if sync.Sync == domain.SyncDirty && !force {
			return domain.WeekDraft{}, fmt.Errorf(
				"dirty draft exists for %s/%s/%s; pass force=true (auto-snapshots) or use refresh",
				profile, weekStart.Format("2006-01-02"), name)
		}
		if sync.Sync == domain.SyncDirty && force {
			if _, err := s.snapshots.Take(existing, OpPrePull, ""); err != nil {
				return domain.WeekDraft{}, fmt.Errorf("auto-snapshot before force pull: %w", err)
			}
		}
	}

	report, err := s.tsvc.GetWeekReport(ctx, profile, weekStart)
	if err != nil {
		return domain.WeekDraft{}, fmt.Errorf("fetch week: %w", err)
	}

	draft := buildDraftFromReport(profile, name, report)
	s.resolveDefaultTimeTypes(ctx, profile, &draft)
	dedupeRowsByKey(&draft)
	if err := s.store.Save(draft); err != nil {
		return domain.WeekDraft{}, err
	}
	if err := s.store.SavePulledSnapshot(draft); err != nil {
		return domain.WeekDraft{}, fmt.Errorf("save pulled snapshot: %w", err)
	}
	return draft, nil
}

// resolveDefaultTimeTypes assigns a default TimeType to each row whose
// TimeType.ID is 0 by querying TD's per-target time-type catalog and
// picking the first valid type. Best-effort: rows whose lookup fails
// keep TimeType.ID=0; the reconcile guard catches them at push time.
//
// Caches lookups by target identity within a single call so repeated
// targets only hit the API once.
func (s *Service) resolveDefaultTimeTypes(ctx context.Context, profile string, draft *domain.WeekDraft) {
	type cacheEntry struct {
		types []domain.TimeType
		hit   bool
	}
	cache := map[string]cacheEntry{}
	for i := range draft.Rows {
		row := &draft.Rows[i]
		if row.TimeType.ID > 0 {
			continue
		}
		key := targetCacheKey(row.Target)
		c, hit := cache[key]
		if !hit {
			t, err := s.tsvc.TimeTypesForTarget(ctx, profile, row.Target)
			if err != nil {
				cache[key] = cacheEntry{types: nil, hit: true}
				continue
			}
			c = cacheEntry{types: t, hit: true}
			cache[key] = c
		}
		if len(c.types) == 0 {
			continue
		}
		row.TimeType = c.types[0]
		// Refresh the row's resolver hints so the editor's TypeName label
		// matches the just-assigned type.
		row.ResolverHints.TimeTypeName = c.types[0].Name
	}
}

// targetCacheKey identifies a target for caching TimeTypesForTarget calls.
// Different target kinds use different ID fields; include them all.
func targetCacheKey(t domain.Target) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d", t.Kind, t.AppID, t.ProjectID, t.ItemID, t.TaskID)
}

// dedupeRowsByKey collapses rows that share (Target, TimeType, Billable).
// When two rows collide (e.g., a real-entry row and a placeholder that
// resolution promoted to the same TimeType), the row with cells wins;
// the empty placeholder is dropped. Stable: input order is preserved
// among kept rows.
func dedupeRowsByKey(draft *domain.WeekDraft) {
	seen := map[string]int{} // key → index in draft.Rows of the kept row
	out := make([]domain.DraftRow, 0, len(draft.Rows))
	for _, row := range draft.Rows {
		k := fmt.Sprintf("%s|%d|%t", targetCacheKey(row.Target), row.TimeType.ID, row.Billable)
		idx, dup := seen[k]
		if !dup {
			seen[k] = len(out)
			out = append(out, row)
			continue
		}
		// Collision: keep the one with cells. If both have cells (shouldn't
		// happen with current pull semantics), keep the first.
		if len(out[idx].Cells) > 0 {
			continue // existing wins
		}
		if len(row.Cells) > 0 {
			out[idx] = row // incoming wins
		}
	}
	// Re-assign sequential row IDs after dedup so they stay contiguous.
	for i := range out {
		out[i].ID = fmt.Sprintf("row-%02d", i+1)
	}
	draft.Rows = out
}

// PulledCellsByKey returns the at-pull-time cells map for sync-state computation.
// Returns an empty map if no pulled snapshot exists (nascent or imported drafts).
func (s *Service) PulledCellsByKey(profile string, weekStart time.Time, name string) (map[string]domain.DraftCell, error) {
	snap, err := s.store.LoadPulledSnapshot(profile, weekStart, name)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]domain.DraftCell{}, nil
		}
		return nil, err
	}
	return pulledCellsByKey(snap), nil
}

// CountConflicts returns the number of conflicted cells (cells whose
// Conflict alt is set) in the named draft. Used by push to refuse on
// SyncConflicted drafts and by resolve to drive its status output.
func (s *Service) CountConflicts(profile string, weekStart time.Time, name string) (int, error) {
	if name == "" {
		name = "default"
	}
	draft, err := s.store.Load(profile, weekStart, name)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, row := range draft.Rows {
		for _, c := range row.Cells {
			if c.Conflict != nil {
				n++
			}
		}
	}
	return n, nil
}

// Reconcile loads current remote state and produces a ReconcileDiff for the
// named draft. userUID is required: it populates EntryInput.UserUID for any
// Create actions. Callers should resolve it via authsvc.WhoAmI.
func (s *Service) Reconcile(ctx context.Context, profile string, weekStart time.Time, name string, userUID string) (domain.WeekDraft, domain.ReconcileDiff, error) {
	if name == "" {
		name = "default"
	}
	draft, err := s.store.Load(profile, weekStart, name)
	if err != nil {
		return domain.WeekDraft{}, domain.ReconcileDiff{}, err
	}

	pulled, err := s.PulledCellsByKey(profile, weekStart, name)
	if err != nil {
		return domain.WeekDraft{}, domain.ReconcileDiff{}, err
	}

	report, err := s.tsvc.GetWeekReport(ctx, profile, weekStart)
	if err != nil {
		return domain.WeekDraft{}, domain.ReconcileDiff{}, err
	}
	locked, err := s.tsvc.GetLockedDays(ctx, profile, weekStart, weekStart.AddDate(0, 0, 6))
	if err != nil {
		return domain.WeekDraft{}, domain.ReconcileDiff{}, err
	}

	diff, err := reconcileDraft(draft, pulled, report, locked, computeRemoteFingerprint(report), userUID)
	if err != nil {
		return draft, domain.ReconcileDiff{}, err
	}
	return draft, diff, nil
}

// ProbeRemoteFingerprint fetches the current remote week report and returns
// its fingerprint. Returns "" on any error, intended for staleness probes
// where failure is non-fatal (the caller renders state without the staleness flag).
func (s *Service) ProbeRemoteFingerprint(ctx context.Context, profile string, weekStart time.Time) string {
	report, err := s.tsvc.GetWeekReport(ctx, profile, weekStart)
	if err != nil {
		return ""
	}
	return computeRemoteFingerprint(report)
}

// pulledCellsByKey extracts the cells-with-source-id map from a draft.
// Used internally by PulledCellsByKey on the loaded pulled snapshot.
func pulledCellsByKey(d domain.WeekDraft) map[string]domain.DraftCell {
	out := map[string]domain.DraftCell{}
	for _, row := range d.Rows {
		for _, cell := range row.Cells {
			if cell.SourceEntryID == 0 {
				continue
			}
			key := fmt.Sprintf("%s:%s", row.ID, cell.Day)
			out[key] = cell
		}
	}
	return out
}
