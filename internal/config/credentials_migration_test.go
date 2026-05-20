package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

// newStoreWithBackends is a test-only constructor that wires a specific
// backend AND keeps yamlBackend available as the migration source. Mirrors
// what the production NewCredentialsStore does at runtime.
func newStoreWithBackends(t *testing.T, backend tokenBackend, paths Paths) *CredentialsStore {
	t.Helper()
	return &CredentialsStore{
		backend:          backend,
		yaml:             newYAMLBackend(paths),
		migrationNoticed: map[string]bool{},
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = oldStderr
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestMigration_HappyPath(t *testing.T) {
	paths := writablePaths(t)

	// Seed YAML directly via yamlBackend.
	yb := newYAMLBackend(paths)
	require.NoError(t, yb.Set("default", "yaml-token"))

	// Construct store with memory backend + yaml fallback.
	mem := newMemoryBackend()
	store := newStoreWithBackends(t, mem, paths)

	var token string
	stderr := captureStderr(t, func() {
		var err error
		token, err = store.GetToken("default")
		require.NoError(t, err)
	})

	// Returned token is the migrated value.
	require.Equal(t, "yaml-token", token)

	// Now in keychain (mem) and absent from YAML.
	memToken, _ := mem.Get("default")
	require.Equal(t, "yaml-token", memToken)

	yamlToken, err := yb.Get("default")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
	_ = yamlToken

	// Stderr notice fired.
	require.Contains(t, stderr, `migrated token for profile "default"`)

	// YAML file removed (was the last entry).
	_, err = os.Stat(filepath.Join(paths.Root, "credentials.yaml"))
	require.True(t, os.IsNotExist(err))
}

func TestMigration_IdempotentSecondCall(t *testing.T) {
	paths := writablePaths(t)
	yb := newYAMLBackend(paths)
	require.NoError(t, yb.Set("default", "yaml-token"))

	mem := newMemoryBackend()
	store := newStoreWithBackends(t, mem, paths)

	// First Get migrates.
	_, err := store.GetToken("default")
	require.NoError(t, err)

	// Second Get should NOT print the notice again.
	stderr := captureStderr(t, func() {
		token, err := store.GetToken("default")
		require.NoError(t, err)
		require.Equal(t, "yaml-token", token)
	})
	require.NotContains(t, stderr, "migrated token")
}

func TestMigration_KeychainSetFailsLeavesYAMLIntact(t *testing.T) {
	paths := writablePaths(t)
	yb := newYAMLBackend(paths)
	require.NoError(t, yb.Set("default", "yaml-token"))

	mem := newMemoryBackend()
	mem.failNextSet = true
	mem.failSetErr = errors.New("simulated keychain Set failure")

	store := newStoreWithBackends(t, mem, paths)

	_, err := store.GetToken("default")
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated keychain Set failure")

	// YAML token untouched — bearer token not lost.
	stillThere, gerr := yb.Get("default")
	require.NoError(t, gerr)
	require.Equal(t, "yaml-token", stillThere)
}

func TestMigration_NoYAMLTokenReturnsErrNoCredentials(t *testing.T) {
	paths := writablePaths(t)
	mem := newMemoryBackend()
	store := newStoreWithBackends(t, mem, paths)

	_, err := store.GetToken("default")
	require.ErrorIs(t, err, domain.ErrNoCredentials)
}

func TestMigration_YAMLBackendDoesNotTriggerMigration(t *testing.T) {
	paths := writablePaths(t)
	yb := newYAMLBackend(paths)
	require.NoError(t, yb.Set("default", "yaml-token"))

	store := newStoreWithBackends(t, yb, paths)

	stderr := captureStderr(t, func() {
		token, err := store.GetToken("default")
		require.NoError(t, err)
		require.Equal(t, "yaml-token", token)
	})
	// No migration notice when backend IS yaml — nothing to migrate.
	require.NotContains(t, stderr, "migrated token")
}

func TestClear_RemovesFromBothBackends(t *testing.T) {
	paths := writablePaths(t)
	yb := newYAMLBackend(paths)
	require.NoError(t, yb.Set("default", "yaml-token"))

	mem := newMemoryBackend()
	require.NoError(t, mem.Set("default", "memory-token"))

	store := newStoreWithBackends(t, mem, paths)
	require.NoError(t, store.ClearToken("default"))

	_, memErr := mem.Get("default")
	require.ErrorIs(t, memErr, domain.ErrNoCredentials)

	_, yamlErr := yb.Get("default")
	require.ErrorIs(t, yamlErr, domain.ErrNoCredentials)
}
