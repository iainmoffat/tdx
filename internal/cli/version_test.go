package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionCommand_PrintsVersionString(t *testing.T) {
	var out bytes.Buffer
	cmd := newVersionCmd("0.1.0-dev")
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.NoError(t, err)
	require.Contains(t, out.String(), "tdx 0.1.0-dev")
}
