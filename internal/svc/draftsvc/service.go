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

// Service is the draft-aware service layer.
type Service struct {
	paths     config.Paths
	store     *Store
	snapshots *SnapshotStore
	tsvc      *timesvc.Service
}

// NewService constructs a Service backed by paths and the live TD time service.
func NewService(paths config.Paths, tsvc *timesvc.Service) *Service {
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
