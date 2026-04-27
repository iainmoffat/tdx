package draftsvc

import (
	"fmt"
	"time"
)

// SetArchived flips the Archived flag on the draft and saves it. Idempotent.
func (s *Service) SetArchived(profile string, weekStart time.Time, name string, archived bool) error {
	if name == "" {
		name = "default"
	}
	d, err := s.store.Load(profile, weekStart, name)
	if err != nil {
		return fmt.Errorf("set archived: %w", err)
	}
	if d.Archived == archived {
		return nil
	}
	d.Archived = archived
	d.ModifiedAt = time.Now().UTC()
	return s.store.Save(d)
}
