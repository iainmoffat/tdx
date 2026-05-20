package config

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/iainmoffat/tdx/internal/domain"
	"gopkg.in/yaml.v3"
)

// credentialsFile is the on-disk shape of credentials.yaml.
type credentialsFile struct {
	Tokens map[string]string `yaml:"tokens"`
}

// yamlBackend persists tokens in ~/.config/tdx/credentials.yaml with 0600 perms.
// This is the fallback backend used when the OS keychain is unavailable or
// explicitly disabled via TDX_TOKEN_BACKEND=yaml.
type yamlBackend struct {
	paths Paths
}

func newYAMLBackend(paths Paths) *yamlBackend {
	return &yamlBackend{paths: paths}
}

func (b *yamlBackend) Name() string { return "yaml" }

func (b *yamlBackend) Get(profile string) (string, error) {
	cf, err := b.load()
	if err != nil {
		return "", err
	}
	token, ok := cf.Tokens[profile]
	if !ok || token == "" {
		return "", fmt.Errorf("%w: %s", domain.ErrNoCredentials, profile)
	}
	return token, nil
}

func (b *yamlBackend) Set(profile, token string) error {
	cf, err := b.load()
	if err != nil {
		return err
	}
	if cf.Tokens == nil {
		cf.Tokens = make(map[string]string)
	}
	cf.Tokens[profile] = token
	return b.save(cf)
}

func (b *yamlBackend) Clear(profile string) error {
	cf, err := b.load()
	if err != nil {
		return err
	}
	if _, ok := cf.Tokens[profile]; !ok {
		return nil
	}
	delete(cf.Tokens, profile)
	if len(cf.Tokens) == 0 {
		// Empty file — remove it entirely. Best-effort.
		_ = os.Remove(b.paths.CredentialsFile)
		return nil
	}
	return b.save(cf)
}

func (b *yamlBackend) load() (credentialsFile, error) {
	data, err := os.ReadFile(b.paths.CredentialsFile)
	if errors.Is(err, os.ErrNotExist) {
		return credentialsFile{Tokens: map[string]string{}}, nil
	}
	if err != nil {
		return credentialsFile{}, fmt.Errorf("read credentials: %w", err)
	}
	if err := enforcePerms(b.paths.CredentialsFile); err != nil {
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

func (b *yamlBackend) save(cf credentialsFile) error {
	if err := b.paths.EnsureRoot(); err != nil {
		return err
	}
	data, err := yaml.Marshal(cf)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	tmp := b.paths.CredentialsFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := os.Rename(tmp, b.paths.CredentialsFile); err != nil {
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
