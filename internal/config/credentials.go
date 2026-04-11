package config

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/ipm/tdx/internal/domain"
	"gopkg.in/yaml.v3"
)

// credentialsFile is the on-disk shape of credentials.yaml.
// Tokens are keyed by profile name.
type credentialsFile struct {
	Tokens map[string]string `yaml:"tokens"`
}

// CredentialsStore persists bearer tokens per profile with 0600 perms.
type CredentialsStore struct {
	paths Paths
}

// NewCredentialsStore constructs a store rooted at the given paths.
func NewCredentialsStore(paths Paths) *CredentialsStore {
	return &CredentialsStore{paths: paths}
}

// GetToken returns the token for the named profile, or ErrNoCredentials.
func (s *CredentialsStore) GetToken(profile string) (string, error) {
	cf, err := s.load()
	if err != nil {
		return "", err
	}
	token, ok := cf.Tokens[profile]
	if !ok || token == "" {
		return "", fmt.Errorf("%w: %s", domain.ErrNoCredentials, profile)
	}
	return token, nil
}

// SetToken writes or replaces the token for the named profile.
func (s *CredentialsStore) SetToken(profile, token string) error {
	cf, err := s.load()
	if err != nil {
		return err
	}
	if cf.Tokens == nil {
		cf.Tokens = make(map[string]string)
	}
	cf.Tokens[profile] = token
	return s.save(cf)
}

// ClearToken removes the token for the named profile. Missing is not an error.
func (s *CredentialsStore) ClearToken(profile string) error {
	cf, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := cf.Tokens[profile]; !ok {
		return nil
	}
	delete(cf.Tokens, profile)
	return s.save(cf)
}

func (s *CredentialsStore) load() (credentialsFile, error) {
	data, err := os.ReadFile(s.paths.CredentialsFile)
	if errors.Is(err, os.ErrNotExist) {
		return credentialsFile{Tokens: map[string]string{}}, nil
	}
	if err != nil {
		return credentialsFile{}, fmt.Errorf("read credentials: %w", err)
	}
	if err := enforcePerms(s.paths.CredentialsFile); err != nil {
		return credentialsFile{}, err
	}
	var cf credentialsFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return credentialsFile{}, fmt.Errorf("parse credentials: %w", err)
	}
	if cf.Tokens == nil {
		cf.Tokens = map[string]string{}
	}
	return cf, nil
}

func (s *CredentialsStore) save(cf credentialsFile) error {
	if err := s.paths.EnsureRoot(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cf)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	tmp := s.paths.CredentialsFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := os.Rename(tmp, s.paths.CredentialsFile); err != nil {
		return fmt.Errorf("finalize credentials: %w", err)
	}
	return nil
}

// enforcePerms narrows the credentials file permissions if they are too open.
// Windows perms are not meaningfully enforceable here, so it is a no-op there.
func enforcePerms(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0o600 {
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("tighten credentials perms: %w", err)
		}
	}
	return nil
}
