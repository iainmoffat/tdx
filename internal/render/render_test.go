package render

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormat_ExplicitJSONWins(t *testing.T) {
	f := ResolveFormat(Flags{JSON: true})
	require.Equal(t, FormatJSON, f)
}

func TestFormat_ExplicitHumanWins(t *testing.T) {
	f := ResolveFormat(Flags{Human: true})
	require.Equal(t, FormatHuman, f)
}

func TestFormat_EnvOverride(t *testing.T) {
	t.Setenv("TDX_FORMAT", "json")
	f := ResolveFormat(Flags{})
	require.Equal(t, FormatJSON, f)
}

func TestFormat_DefaultsToHuman(t *testing.T) {
	t.Setenv("TDX_FORMAT", "")
	f := ResolveFormat(Flags{})
	require.Equal(t, FormatHuman, f)
}

func TestJSON_EncodesPrettily(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, JSON(&buf, map[string]any{"hello": "world"}))
	require.Contains(t, buf.String(), "\"hello\": \"world\"")
}

func TestHuman_WritesRawLines(t *testing.T) {
	var buf bytes.Buffer
	Humanf(&buf, "profile: %s", "default")
	require.Equal(t, "profile: default\n", buf.String())
}
