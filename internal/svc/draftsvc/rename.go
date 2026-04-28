package draftsvc

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
)

// Rename moves a draft from oldName to newName at the same (profile, weekStart).
// Auto-snapshots before any file motion. Refuses on collision. Renames the YAML,
// the pulled-snapshot sibling, and the snapshots directory.
func (s *Service) Rename(profile string, weekStart time.Time, oldName, newName string) error {
	if oldName == newName {
		return fmt.Errorf("rename: oldName == newName")
	}
	if !s.store.Exists(profile, weekStart, oldName) {
		return fmt.Errorf("rename: draft %q does not exist", oldName)
	}
	if s.store.Exists(profile, weekStart, newName) {
		return fmt.Errorf("rename: target %q already exists", newName)
	}

	src, err := s.store.Load(profile, weekStart, oldName)
	if err != nil {
		return err
	}

	if _, err := s.snapshots.Take(src, OpPreRename, "rename:"+oldName+"->"+newName); err != nil {
		return fmt.Errorf("rename: pre-flight snapshot: %w", err)
	}

	dateDir := weekStart.In(domain.EasternTZ).Format("2006-01-02")
	weeksDir := s.paths.ProfileWeeksDir(profile)
	oldYAML := filepath.Join(weeksDir, dateDir, oldName+".yaml")
	oldPulled := filepath.Join(weeksDir, dateDir, oldName+".pulled.yaml")
	newPulled := filepath.Join(weeksDir, dateDir, newName+".pulled.yaml")
	oldSnaps := filepath.Join(weeksDir, dateDir, oldName+".snapshots")
	newSnaps := filepath.Join(weeksDir, dateDir, newName+".snapshots")

	src.Name = newName
	if err := s.store.Save(src); err != nil {
		return fmt.Errorf("rename: write new YAML: %w", err)
	}
	if err := os.Remove(oldYAML); err != nil {
		return fmt.Errorf("rename: remove old YAML: %w", err)
	}

	if _, err := os.Stat(oldPulled); err == nil {
		if err := os.Rename(oldPulled, newPulled); err != nil {
			return fmt.Errorf("rename: pulled sibling: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("rename: stat pulled sibling: %w", err)
	}

	if _, err := os.Stat(oldSnaps); err == nil {
		if err := os.Rename(oldSnaps, newSnaps); err != nil {
			return fmt.Errorf("rename: snapshots dir: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("rename: stat snapshots dir: %w", err)
	}

	return nil
}
