package config

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"

	"github.com/iainmoffat/tdx/internal/domain"
)

// keychainServiceName is the service identifier under which tdx tokens are
// stored. On macOS this appears in Keychain Access.app as the "Where" field;
// on Linux it's the secret service name; on Windows it's the Credential
// Manager target name (combined with the profile as the account).
const keychainServiceName = "tdx"

// keychainBackend wraps zalando/go-keyring. Each profile's token is stored
// under service=tdx, account=<profile-name>.
type keychainBackend struct{}

func newKeychainBackend() *keychainBackend { return &keychainBackend{} }

func (b *keychainBackend) Name() string { return "keychain" }

func (b *keychainBackend) Get(profile string) (string, error) {
	t, err := keyring.Get(keychainServiceName, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", fmt.Errorf("%w: %s", domain.ErrNoCredentials, profile)
	}
	if err != nil {
		return "", fmt.Errorf("keychain get: %w", err)
	}
	return t, nil
}

func (b *keychainBackend) Set(profile, token string) error {
	if err := keyring.Set(keychainServiceName, profile, token); err != nil {
		return fmt.Errorf("keychain set: %w", err)
	}
	return nil
}

func (b *keychainBackend) Clear(profile string) error {
	err := keyring.Delete(keychainServiceName, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("keychain clear: %w", err)
	}
	return nil
}
