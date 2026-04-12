package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompletionBash(t *testing.T) {
	root := NewRootCmd("test")
	root.SetArgs([]string{"completion", "bash"})
	var out bytes.Buffer
	root.SetOut(&out)
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "bash completion")
}

func TestCompletionZsh(t *testing.T) {
	root := NewRootCmd("test")
	root.SetArgs([]string{"completion", "zsh"})
	var out bytes.Buffer
	root.SetOut(&out)
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "zsh completion")
}

func TestCompletionFish(t *testing.T) {
	root := NewRootCmd("test")
	root.SetArgs([]string{"completion", "fish"})
	var out bytes.Buffer
	root.SetOut(&out)
	require.NoError(t, root.Execute())
	// Fish completions contain the binary name.
	require.Contains(t, out.String(), "tdx")
}
