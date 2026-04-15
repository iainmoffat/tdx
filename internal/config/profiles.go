package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/iainmoffat/tdx/internal/domain"
	"gopkg.in/yaml.v3"
)

// ProfileConfig is the on-disk shape of config.yaml.
type ProfileConfig struct {
	DefaultProfile string           `yaml:"defaultProfile"`
	Profiles       []domain.Profile `yaml:"profiles"`
}

// ProfileStore reads and writes the profile configuration file.
type ProfileStore struct {
	paths Paths
}

// NewProfileStore constructs a store rooted at the given paths.
func NewProfileStore(paths Paths) *ProfileStore {
	return &ProfileStore{paths: paths}
}

// Load returns the current profile config, or an empty config if none exists.
func (s *ProfileStore) Load() (ProfileConfig, error) {
	data, err := os.ReadFile(s.paths.ConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		return ProfileConfig{}, nil
	}
	if err != nil {
		return ProfileConfig{}, fmt.Errorf("read config: %w", err)
	}
	var cfg ProfileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ProfileConfig{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// Save writes the config atomically (write temp, rename).
func (s *ProfileStore) Save(cfg ProfileConfig) error {
	if err := s.paths.EnsureRoot(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := s.paths.ConfigFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, s.paths.ConfigFile); err != nil {
		return fmt.Errorf("finalize config: %w", err)
	}
	return nil
}

// AddProfile inserts a new profile; rejects name collisions.
// The first profile added becomes the default.
func (s *ProfileStore) AddProfile(p domain.Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	cfg, err := s.Load()
	if err != nil {
		return err
	}
	for _, existing := range cfg.Profiles {
		if existing.Name == p.Name {
			return fmt.Errorf("%w: %s", domain.ErrProfileExists, p.Name)
		}
	}
	cfg.Profiles = append(cfg.Profiles, p)
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = p.Name
	}
	return s.Save(cfg)
}

// RemoveProfile deletes a profile by name.
// If the removed profile was the default, another remaining profile becomes default.
func (s *ProfileStore) RemoveProfile(name string) error {
	cfg, err := s.Load()
	if err != nil {
		return err
	}
	idx := -1
	for i, p := range cfg.Profiles {
		if p.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("%w: %s", domain.ErrProfileNotFound, name)
	}
	cfg.Profiles = append(cfg.Profiles[:idx], cfg.Profiles[idx+1:]...)
	if cfg.DefaultProfile == name {
		if len(cfg.Profiles) > 0 {
			cfg.DefaultProfile = cfg.Profiles[0].Name
		} else {
			cfg.DefaultProfile = ""
		}
	}
	return s.Save(cfg)
}

// GetProfile returns a profile by name, or ErrProfileNotFound.
func (s *ProfileStore) GetProfile(name string) (domain.Profile, error) {
	cfg, err := s.Load()
	if err != nil {
		return domain.Profile{}, err
	}
	for _, p := range cfg.Profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return domain.Profile{}, fmt.Errorf("%w: %s", domain.ErrProfileNotFound, name)
}

// SetDefault sets the default profile, verifying it exists.
func (s *ProfileStore) SetDefault(name string) error {
	cfg, err := s.Load()
	if err != nil {
		return err
	}
	found := false
	for _, p := range cfg.Profiles {
		if p.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%w: %s", domain.ErrProfileNotFound, name)
	}
	cfg.DefaultProfile = name
	return s.Save(cfg)
}
