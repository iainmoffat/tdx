package render

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormat_ExplicitJSONWins(t *testing.T) {
	f := ResolveFormat(Flags{JSON: true}, os.Stdout)
	require.Equal(t, FormatJSON, f)
}

func TestFormat_ExplicitHumanWins(t *testing.T) {
	f := ResolveFormat(Flags{Human: true}, os.Stdout)
	require.Equal(t, FormatHuman, f)
}

func TestFormat_EnvOverride(t *testing.T) {
	t.Setenv("TDX_FORMAT", "json")
	f := ResolveFormat(Flags{}, os.Stdout)
	require.Equal(t, FormatJSON, f)
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
