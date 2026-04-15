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

// Store manages template YAML files on disk under config.Paths.TemplatesDir.
type Store struct {
	dir string
}

// NewStore constructs a Store rooted at paths.TemplatesDir.
func NewStore(paths config.Paths) *Store {
	return &Store{dir: paths.TemplatesDir}
}

// path returns the full file path for a template with the given name.
func (s *Store) path(name string) string {
	return filepath.Join(s.dir, name+".yaml")
}

// Save marshals tmpl to YAML and writes it to <TemplatesDir>/<tmpl.Name>.yaml.
// The templates directory is created if it does not exist.
func (s *Store) Save(tmpl domain.Template) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create templates dir: %w", err)
	}
	data, err := yaml.Marshal(tmpl)
	if err != nil {
		return fmt.Errorf("marshal template %q: %w", tmpl.Name, err)
	}
	if err := os.WriteFile(s.path(tmpl.Name), data, 0o600); err != nil {
		return fmt.Errorf("write template %q: %w", tmpl.Name, err)
	}
	return nil
}

// Load reads and unmarshals the template with the given name.
// Returns a descriptive error containing "not found" when the file is absent.
func (s *Store) Load(name string) (domain.Template, error) {
	data, err := os.ReadFile(s.path(name))
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Template{}, fmt.Errorf("template %q not found", name)
		}
		return domain.Template{}, fmt.Errorf("read template %q: %w", name, err)
	}
	var tmpl domain.Template
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return domain.Template{}, fmt.Errorf("unmarshal template %q: %w", name, err)
	}
	return tmpl, nil
}

// List returns all templates stored in the templates directory.
// Returns nil, nil when the directory is absent or empty.
func (s *Store) List() ([]domain.Template, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read templates dir: %w", err)
	}

	var templates []domain.Template
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		tmpl, err := s.Load(name)
		if err != nil {
			return nil, fmt.Errorf("load template %q: %w", name, err)
		}
		templates = append(templates, tmpl)
	}
	return templates, nil
}

// Delete removes the YAML file for the named template.
// Returns a descriptive error containing "not found" when the file is absent.
func (s *Store) Delete(name string) error {
	err := os.Remove(s.path(name))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("template %q not found", name)
		}
		return fmt.Errorf("delete template %q: %w", name, err)
	}
	return nil
}

// Exists reports whether a YAML file exists for the named template.
func (s *Store) Exists(name string) bool {
	_, err := os.Stat(s.path(name))
	return err == nil
}
