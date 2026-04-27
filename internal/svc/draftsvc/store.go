package draftsvc

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

// Store persists week drafts as YAML files under
// <root>/profiles/<profile>/weeks/<weekStart>/<name>.yaml.
type Store struct {
	paths config.Paths
}

// NewStore constructs a Store rooted at paths.
func NewStore(paths config.Paths) *Store { return &Store{paths: paths} }

func (s *Store) draftPath(profile string, weekStart time.Time, name string) string {
	dateDir := weekStart.In(domain.EasternTZ).Format("2006-01-02")
	return filepath.Join(s.paths.ProfileWeeksDir(profile), dateDir, name+".yaml")
}

// Save writes the draft to disk. Creates parent directories as needed.
func (s *Store) Save(d domain.WeekDraft) error {
	if err := d.Validate(); err != nil {
		return fmt.Errorf("validate draft: %w", err)
	}
	p := s.draftPath(d.Profile, d.WeekStart, d.Name)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	data, err := yaml.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", p, err)
	}
	return nil
}

// Load reads the draft. Returns a "not found" error if the file is absent.
func (s *Store) Load(profile string, weekStart time.Time, name string) (domain.WeekDraft, error) {
	p := s.draftPath(profile, weekStart, name)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.WeekDraft{}, fmt.Errorf("draft not found: %s/%s/%s",
				profile, weekStart.In(domain.EasternTZ).Format("2006-01-02"), name)
		}
		return domain.WeekDraft{}, err
	}
	var d domain.WeekDraft
	if err := yaml.Unmarshal(data, &d); err != nil {
		return domain.WeekDraft{}, fmt.Errorf("unmarshal %s: %w", p, err)
	}
	return d, nil
}

// Exists reports whether a draft exists at (profile, weekStart, name).
func (s *Store) Exists(profile string, weekStart time.Time, name string) bool {
	_, err := os.Stat(s.draftPath(profile, weekStart, name))
	return err == nil
}

// Delete removes the draft file. Snapshots beside it are NOT removed.
func (s *Store) Delete(profile string, weekStart time.Time, name string) error {
	p := s.draftPath(profile, weekStart, name)
	if err := os.Remove(p); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("draft not found: %s/%s/%s",
				profile, weekStart.In(domain.EasternTZ).Format("2006-01-02"), name)
		}
		return err
	}
	return nil
}

// List returns all drafts for the given profile, ordered by (weekStart desc, name asc).
func (s *Store) List(profile string) ([]domain.WeekDraft, error) {
	root := s.paths.ProfileWeeksDir(profile)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var drafts []domain.WeekDraft
	for _, dateEntry := range entries {
		if !dateEntry.IsDir() {
			continue
		}
		weekStart, err := time.ParseInLocation("2006-01-02", dateEntry.Name(), domain.EasternTZ)
		if err != nil {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, dateEntry.Name()))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".yaml") {
				continue
			}
			// Skip pulled-snapshot sibling files (Task 8 will use ".pulled.yaml" suffix).
			if strings.HasSuffix(f.Name(), ".pulled.yaml") {
				continue
			}
			name := strings.TrimSuffix(f.Name(), ".yaml")
			d, err := s.Load(profile, weekStart, name)
			if err != nil {
				return nil, err
			}
			drafts = append(drafts, d)
		}
	}
	sort.SliceStable(drafts, func(i, j int) bool {
		if !drafts[i].WeekStart.Equal(drafts[j].WeekStart) {
			return drafts[i].WeekStart.After(drafts[j].WeekStart)
		}
		return drafts[i].Name < drafts[j].Name
	})
	return drafts, nil
}
