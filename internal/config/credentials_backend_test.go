package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectBackend_DefaultIsYAML(t *testing.T) {
	t.Setenv("TDX_TOKEN_BACKEND", "")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
}

func TestSelectBackend_AutoIsYAML(t *testing.T) {
	// Until Task 4, auto falls back to yaml.
	t.Setenv("TDX_TOKEN_BACKEND", "auto")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
}

func TestSelectBackend_YAMLForced(t *testing.T) {
	t.Setenv("TDX_TOKEN_BACKEND", "yaml")
	b, err := selectBackend(writablePaths(t))
	require.NoError(t, err)
	require.Equal(t, "yaml", b.Name())
}

func TestSelectBackend_KeychainErrorsUntilTask4(t *testing.T) {
	t.Setenv("TDX_TOKEN_BACKEND", "keychain")
	_, err := selectBackend(writablePaths(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "keychain")
}

func TestSelectBackend_InvalidValue(t *testing.T) {
	t.Setenv("TDX_TOKEN_BACKEND", "bogus")
	_, err := selectBackend(writablePaths(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid TDX_TOKEN_BACKEND")
}
