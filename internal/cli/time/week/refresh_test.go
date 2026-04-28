package week

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
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

func TestNewRebaseCmd_IsAliasOfRefresh(t *testing.T) {
	cmd := newRebaseCmd()
	require.NotNil(t, cmd)
	require.Equal(t, "rebase <date>[/<name>]", cmd.Use)
	require.NotNil(t, cmd.Flags().Lookup("strategy"))
	require.NotNil(t, cmd.Flags().Lookup("profile"))
	require.NotNil(t, cmd.Flags().Lookup("json"))
	def, err := cmd.Flags().GetString("strategy")
	require.NoError(t, err)
	require.Equal(t, "abort", def)
}

func TestWriteRefreshJSON_AbortShape(t *testing.T) {
	var buf bytes.Buffer
	res := draftsvc.RefreshResult{
		Strategy: draftsvc.StrategyAbort,
		Aborted:  true,
		Conflicts: []draftsvc.MergeConflict{
			{RowID: "row-01", Day: "Monday", LocalDescription: "updated to 6.0h", RemoteDescription: "updated to 8.0h"},
		},
	}
	err := writeRefreshJSON(&buf, time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), "default", res)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, "tdx.v1.weekDraftRefreshResult", got["schema"])
	require.Equal(t, "abort", got["strategy"])
	require.Equal(t, true, got["aborted"])
	require.Equal(t, float64(0), got["adopted"])
	require.Equal(t, float64(0), got["preserved"])
	require.Equal(t, float64(0), got["resolved"])
	require.Equal(t, float64(0), got["resolvedByStrategy"])
	conflicts, ok := got["conflicts"].([]any)
	require.True(t, ok, "conflicts should be an array")
	require.Len(t, conflicts, 1)
	c, ok := conflicts[0].(map[string]any)
	require.True(t, ok, "conflict[0] should be an object")
	require.Equal(t, "row-01", c["row"])
	require.Equal(t, "Monday", c["day"])
	require.Equal(t, "updated to 6.0h", c["local"])
	require.Equal(t, "updated to 8.0h", c["remote"])
}

func TestWriteRefreshJSON_SuccessShape(t *testing.T) {
	var buf bytes.Buffer
	res := draftsvc.RefreshResult{
		Strategy:           draftsvc.StrategyOurs,
		Adopted:            3,
		Preserved:          5,
		Resolved:           1,
		ResolvedByStrategy: 2,
	}
	err := writeRefreshJSON(&buf, time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), "default", res)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, "ours", got["strategy"])
	require.Equal(t, false, got["aborted"])
	require.Equal(t, float64(3), got["adopted"])
	require.Equal(t, float64(5), got["preserved"])
	require.Equal(t, float64(1), got["resolved"])
	require.Equal(t, float64(2), got["resolvedByStrategy"])
	conflicts, ok := got["conflicts"].([]any)
	require.True(t, ok || got["conflicts"] == nil, "conflicts is array or absent")
	require.Empty(t, conflicts)
}
