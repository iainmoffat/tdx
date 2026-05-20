package mcp

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMain defaults TDX_TOKEN_BACKEND=yaml for this package so tests never
// touch the dev's real OS keychain.
func TestMain(m *testing.M) {
	if os.Getenv("TDX_TOKEN_BACKEND") == "" {
		os.Setenv("TDX_TOKEN_BACKEND", "yaml")
	}
	os.Exit(m.Run())
}

func TestConfirmGate_NotConfirmed(t *testing.T) {
	result, ok := confirmGate(false, "Please confirm.")
	require.False(t, ok)
	require.NotNil(t, result)
	require.True(t, result.IsError)
}

func TestConfirmGate_Confirmed(t *testing.T) {
	result, ok := confirmGate(true, "Please confirm.")
	require.True(t, ok)
	require.Nil(t, result)
}

func TestJsonResult(t *testing.T) {
	result, _, err := jsonResult(map[string]string{"key": "value"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)
}

func TestResolveProfile(t *testing.T) {
	svcs := Services{Profile: "default"}
	require.Equal(t, "default", resolveProfile(svcs, ""))
	require.Equal(t, "custom", resolveProfile(svcs, "custom"))
}
