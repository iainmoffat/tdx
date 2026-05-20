package config

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectBackend_DefaultIsYAMLWhenProbeFails(t *testing.T) {
	originalProbe := probeKeychain
	probeKeychain = func() error { return errors.New("forced fallback") }
	t.Cleanup(func() { probeKeychain = originalProbe })

	t.Setenv("TDX_TOKEN_BACKEND", "")
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	b, err := selectBackend(writablePaths(t))
	_ = w.Close()
	os.Stderr = oldStderr
	_, _ = io.ReadAll(r)

	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
}

func TestSelectBackend_AutoIsYAMLWhenProbeFails(t *testing.T) {
	originalProbe := probeKeychain
	probeKeychain = func() error { return errors.New("forced fallback") }
	t.Cleanup(func() { probeKeychain = originalProbe })

	t.Setenv("TDX_TOKEN_BACKEND", "auto")
	// Capture stderr so the notice doesn't leak into test output.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	b, err := selectBackend(writablePaths(t))
	_ = w.Close()
	os.Stderr = oldStderr
	_, _ = io.ReadAll(r)

	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
}

func TestSelectBackend_YAMLForced(t *testing.T) {
	t.Setenv("TDX_TOKEN_BACKEND", "yaml")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
}

func TestSelectBackend_InvalidValue(t *testing.T) {
	t.Setenv("TDX_TOKEN_BACKEND", "bogus")
	_, err := selectBackend(writablePaths(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid TDX_TOKEN_BACKEND")
}

func TestSelectBackend_KeychainStrictFailsClosedWhenProbeFails(t *testing.T) {
	// Swap out the probe to simulate keychain unavailable.
	originalProbe := probeKeychain
	probeKeychain = func() error { return errors.New("simulated keychain down") }
	t.Cleanup(func() { probeKeychain = originalProbe })

	t.Setenv("TDX_TOKEN_BACKEND", "keychain")
	_, err := selectBackend(writablePaths(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "keychain is unavailable")
}

func TestSelectBackend_KeychainStrictSucceedsWhenProbeOK(t *testing.T) {
	originalProbe := probeKeychain
	probeKeychain = func() error { return nil }
	t.Cleanup(func() { probeKeychain = originalProbe })

	t.Setenv("TDX_TOKEN_BACKEND", "keychain")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "keychain", b.Name())
}

func TestSelectBackend_AutoFallsBackToYAMLWhenProbeFails(t *testing.T) {
	originalProbe := probeKeychain
	probeKeychain = func() error { return errors.New("simulated keychain down") }
	t.Cleanup(func() { probeKeychain = originalProbe })

	// Capture stderr to assert the notice.
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	t.Setenv("TDX_TOKEN_BACKEND", "auto")
	b, err := selectBackend(writablePaths(t))

	_ = w.Close()
	os.Stderr = oldStderr
	stderr, _ := io.ReadAll(r)

	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
	require.Contains(t, string(stderr), "notice: keychain unavailable")
}

func TestSelectBackend_AutoUsesKeychainWhenProbeOK(t *testing.T) {
	originalProbe := probeKeychain
	probeKeychain = func() error { return nil }
	t.Cleanup(func() { probeKeychain = originalProbe })

	t.Setenv("TDX_TOKEN_BACKEND", "auto")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "keychain", b.Name())
}
