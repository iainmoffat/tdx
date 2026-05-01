package week

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/svc/draftsvc"
)

func TestNewResolveCmd_FlagsRegistered(t *testing.T) {
	cmd := newResolveCmd()
	require.NotNil(t, cmd)
	require.Equal(t, "resolve [date[/name]]", cmd.Use)
	for _, name := range []string{"profile", "row", "day", "pick", "all-local", "all-remote", "yes", "json"} {
		require.NotNil(t, cmd.Flags().Lookup(name), "missing flag: %s", name)
	}
}

func TestValidateResolveFlags_OK(t *testing.T) {
	cases := []resolveFlags{
		{},                                       // bare → status
		{allLocal: true},                         // bulk
		{allRemote: true, yes: true},             // bulk with yes
		{row: "r", day: "Monday", pick: "local"}, // per-cell
		{row: "r", day: "monday", pick: "REMOTE", yes: true},
	}
	for i, f := range cases {
		require.NoError(t, validateResolveFlags(f), "case %d", i)
	}
}

func TestValidateResolveFlags_TripleRequired(t *testing.T) {
	cases := []resolveFlags{
		{row: "r"},
		{day: "Monday"},
		{pick: "local"},
		{row: "r", day: "Monday"},
		{row: "r", pick: "local"},
		{day: "Monday", pick: "local"},
	}
	for i, f := range cases {
		err := validateResolveFlags(f)
		require.Error(t, err, "case %d", i)
		require.Contains(t, err.Error(), "must be given together")
	}
}

func TestValidateResolveFlags_AllLocalAndRemoteMutex(t *testing.T) {
	err := validateResolveFlags(resolveFlags{allLocal: true, allRemote: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestValidateResolveFlags_BulkAndPerCellMutex(t *testing.T) {
	err := validateResolveFlags(resolveFlags{
		allLocal: true,
		row:      "r", day: "Monday", pick: "local",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
}

func TestWriteConflictsJSON_Shape(t *testing.T) {
	var buf bytes.Buffer
	conflicts := []draftsvc.Conflict{
		{
			RowID: "row-01", Day: time.Monday,
			LocalHours: 6, LocalSrcID: 900,
			RemoteHours: 8, RemoteSrcID: 900,
			PulledHours: 4,
		},
		{
			RowID: "row-01", Day: time.Tuesday,
			LocalHours: 6, LocalSrcID: 901,
			RemoteHours: 0, RemoteSrcID: 0,
			PulledHours:   4,
			RemoteDeletes: true,
		},
	}
	err := writeConflictsJSON(&buf, time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), "default", conflicts)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, "tdx.v1.weekDraftConflicts", got["schema"])
	require.Equal(t, "2026-05-03", got["weekStart"])
	arr, ok := got["conflicts"].([]any)
	require.True(t, ok)
	require.Len(t, arr, 2)
	first, ok := arr[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "row-01", first["rowID"])
	require.Equal(t, "Monday", first["day"])
	require.Equal(t, float64(6), first["localHours"])
	require.Equal(t, float64(8), first["remoteHours"])
	second, ok := arr[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, second["remoteDeletes"])
}

func TestWriteResolveJSON_Shape(t *testing.T) {
	var buf bytes.Buffer
	res := draftsvc.ResolveResult{
		PicksApplied: 3, PickedLocal: 1, PickedRemote: 2,
		DroppedDeletedCells: 1, ConflictsRemaining: 0,
	}
	err := writeResolveJSON(&buf, time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), "default", res)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, "tdx.v1.weekDraftResolveResult", got["schema"])
	require.Equal(t, float64(3), got["picksApplied"])
	require.Equal(t, float64(1), got["pickedLocal"])
	require.Equal(t, float64(2), got["pickedRemote"])
	require.Equal(t, float64(1), got["droppedDeletedCells"])
	require.Equal(t, float64(0), got["conflictsRemaining"])
}

func TestWriteConflictsText_EmptyMessage(t *testing.T) {
	var buf bytes.Buffer
	writeConflictsText(&buf, time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), nil)
	require.Contains(t, buf.String(), "no conflicts")
}

func TestWriteConflictsText_TableAndHints(t *testing.T) {
	var buf bytes.Buffer
	conflicts := []draftsvc.Conflict{
		{RowID: "row-01", Day: time.Monday, LocalHours: 6, LocalSrcID: 900, RemoteHours: 8, RemoteSrcID: 900, PulledHours: 4},
	}
	writeConflictsText(&buf, time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC), conflicts)
	s := buf.String()
	require.Contains(t, s, "conflicts: 1")
	require.Contains(t, s, "row-01")
	require.Contains(t, s, "--all-local")
	require.Contains(t, s, "--all-remote")
}
