package draftsvc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// Reset discards local edits and re-pulls the week. Requires the existing
// draft to load successfully; takes a pre-reset snapshot before clobbering.
func (s *Service) Reset(ctx context.Context, profile string, weekStart time.Time, name string) error {
	if name == "" {
		name = "default"
	}
	existing, err := s.store.Load(profile, weekStart, name)
	if err != nil {
		return fmt.Errorf("reset: load existing: %w", err)
	}
	if _, err := s.snapshots.Take(existing, OpPreReset, ""); err != nil {
		return fmt.Errorf("reset: snapshot: %w", err)
	}

	if err := s.store.Delete(profile, weekStart, name); err != nil {
		return fmt.Errorf("reset: delete existing: %w", err)
	}
	dateDir := weekStart.In(domain.EasternTZ).Format("2006-01-02")
	pulled := filepath.Join(s.paths.ProfileWeeksDir(profile), dateDir, name+".pulled.yaml")
	_ = os.Remove(pulled)

	if _, err := s.Pull(ctx, profile, weekStart, name, false); err != nil {
		return fmt.Errorf("reset: re-pull: %w", err)
	}
	return nil
}
