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
	if err := s.store.Save(draft); err != nil {
		return domain.WeekDraft{}, err
	}
	if err := s.store.SavePulledSnapshot(draft); err != nil {
		return domain.WeekDraft{}, fmt.Errorf("save pulled snapshot: %w", err)
	}
	return draft, nil
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
