package week

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRefreshCmd_FlagsRegistered(t *testing.T) {
	cmd := newRefreshCmd()
	require.NotNil(t, cmd)
	require.Equal(t, "refresh <date>[/<name>]", cmd.Use)
	require.NotNil(t, cmd.Flags().Lookup("strategy"), "--strategy flag missing")
	require.NotNil(t, cmd.Flags().Lookup("profile"), "--profile flag missing")
	require.NotNil(t, cmd.Flags().Lookup("json"), "--json flag missing")
	def, err := cmd.Flags().GetString("strategy")
	require.NoError(t, err)
	require.Equal(t, "abort", def, "default strategy must be abort")
}
