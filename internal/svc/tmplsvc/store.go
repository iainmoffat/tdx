package tmplsvc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

// Store manages template YAML files. Templates are scoped per-profile under
// <root>/profiles/<profile>/templates/. The legacy global <root>/templates/
// path is consulted as a fallback for reads when a per-profile lookup misses,
// covering the brief window before the templates-per-profile migration runs.
type Store struct {
	paths config.Paths
}

// NewStore constructs a Store rooted at paths.
func NewStore(paths config.Paths) *Store {
	return &Store{paths: paths}
}

func (s *Store) profilePath(profile, name string) string {
	return filepath.Join(s.paths.ProfileTemplatesDir(profile), name+".yaml")
}

func (s *Store) legacyPath(name string) string {
	if s.paths.LegacyTemplatesDir == "" {
		return ""
	}
	return filepath.Join(s.paths.LegacyTemplatesDir, name+".yaml")
}

// Save writes a template into the per-profile templates directory.
func (s *Store) Save(profile string, tmpl domain.Template) error {
	dir := s.paths.ProfileTemplatesDir(profile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create templates dir: %w", err)
	}
	data, err := yaml.Marshal(tmpl)
	if err != nil {
		return fmt.Errorf("marshal template %q: %w", tmpl.Name, err)
	}
	if err := os.WriteFile(s.profilePath(profile, tmpl.Name), data, 0o600); err != nil {
		return fmt.Errorf("write template %q: %w", tmpl.Name, err)
	}
	return nil
}

// Load reads a template by name. Looks in the per-profile dir first, then
// falls back to the legacy global dir.
func (s *Store) Load(profile, name string) (domain.Template, error) {
	if data, err := os.ReadFile(s.profilePath(profile, name)); err == nil {
		var tmpl domain.Template
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			return domain.Template{}, fmt.Errorf("unmarshal template %q: %w", name, err)
		}
		return tmpl, nil
	} else if !os.IsNotExist(err) {
		return domain.Template{}, fmt.Errorf("read template %q: %w", name, err)
	}
	if lp := s.legacyPath(name); lp != "" {
		if data, err := os.ReadFile(lp); err == nil {
			var tmpl domain.Template
			if err := yaml.Unmarshal(data, &tmpl); err != nil {
				return domain.Template{}, fmt.Errorf("unmarshal template %q: %w", name, err)
			}
			return tmpl, nil
		} else if !os.IsNotExist(err) {
			return domain.Template{}, fmt.Errorf("read template %q (legacy): %w", name, err)
		}
	}
	return domain.Template{}, fmt.Errorf("template %q not found", name)
}

// Exists reports whether a template exists at the per-profile or legacy path.
func (s *Store) Exists(profile, name string) bool {
	if _, err := os.Stat(s.profilePath(profile, name)); err == nil {
		return true
	}
	if lp := s.legacyPath(name); lp != "" {
		if _, err := os.Stat(lp); err == nil {
			return true
		}
	}
	return false
}

// Delete removes a template. Tries per-profile first; falls back to legacy if
// not found there. Returns an error if the template doesn't exist in either.
func (s *Store) Delete(profile, name string) error {
	if err := os.Remove(s.profilePath(profile, name)); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("delete template %q: %w", name, err)
	}
	if lp := s.legacyPath(name); lp != "" {
		if err := os.Remove(lp); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("delete template %q (legacy): %w", name, err)
		}
	}
	return fmt.Errorf("template %q not found", name)
}

// List returns all templates visible to the profile. Reads from the per-profile
// dir AND the legacy dir; per-profile entries shadow legacy entries with the
// same name.
func (s *Store) List(profile string) ([]domain.Template, error) {
	seen := map[string]struct{}{}
	var out []domain.Template

	perProfile := s.paths.ProfileTemplatesDir(profile)
	if entries, err := os.ReadDir(perProfile); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".yaml")
			t, err := s.Load(profile, name)
			if err != nil {
				return nil, fmt.Errorf("load template %q: %w", name, err)
			}
			seen[name] = struct{}{}
			out = append(out, t)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read templates dir: %w", err)
	}

	if s.paths.LegacyTemplatesDir != "" {
		if entries, err := os.ReadDir(s.paths.LegacyTemplatesDir); err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
					continue
				}
				name := strings.TrimSuffix(e.Name(), ".yaml")
				if _, dup := seen[name]; dup {
					continue
				}
				t, err := s.Load(profile, name)
				if err != nil {
					return nil, fmt.Errorf("load legacy template %q: %w", name, err)
				}
				out = append(out, t)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read legacy templates dir: %w", err)
		}
	}
	return out, nil
}
