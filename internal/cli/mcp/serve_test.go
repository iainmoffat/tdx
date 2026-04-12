package mcp

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestServeCmd_Help verifies that the `mcp serve` sub-command is properly wired
// and reports the expected short description in its help output.
func TestServeCmd_Help(t *testing.T) {
	cmd := NewCmd()
	cmd.SetArgs([]string{"serve", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, out.String(), "Start the MCP server")
}
