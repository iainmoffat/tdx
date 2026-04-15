package timesvc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestTdTime_UnmarshalRFC3339Z(t *testing.T) {
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`"2026-04-06T00:00:00Z"`), &got))
	require.Equal(t, 2026, got.Year())
	require.Equal(t, 4, int(got.Month()))
	require.Equal(t, 6, got.Day())
	require.Equal(t, time.UTC, got.Location())
}

func TestTdTime_UnmarshalRFC3339Nano(t *testing.T) {
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`"2026-04-06T15:22:01.607Z"`), &got))
	require.Equal(t, 15, got.Hour())
	require.Equal(t, 22, got.Minute())
	require.Equal(t, 607000000, got.Nanosecond())
	require.Equal(t, time.UTC, got.Location())
}

func TestTdTime_UnmarshalNoZoneWithFractional(t *testing.T) {
	// This is the format that broke the Phase 2 walkthrough.
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`"2026-04-03T15:22:01.607"`), &got))
	require.Equal(t, 2026, got.Year())
	require.Equal(t, 4, int(got.Month()))
	require.Equal(t, 3, got.Day())
	require.Equal(t, 15, got.Hour())
	require.Equal(t, 22, got.Minute())
	require.Equal(t, 1, got.Second())
	// Wall-clock value with no zone is interpreted as EasternTZ.
	require.Equal(t, domain.EasternTZ, got.Location())
}

func TestTdTime_UnmarshalNoZoneNoFractional(t *testing.T) {
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`"2026-04-03T15:22:01"`), &got))
	require.Equal(t, 15, got.Hour())
	require.Equal(t, 22, got.Minute())
	require.Equal(t, 1, got.Second())
	require.Equal(t, domain.EasternTZ, got.Location())
}

func TestTdTime_UnmarshalEmpty(t *testing.T) {
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`""`), &got))
	require.True(t, got.IsZero())
}

func TestTdTime_UnmarshalNull(t *testing.T) {
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`null`), &got))
	require.True(t, got.IsZero())
}

func TestTdTime_UnmarshalGarbage(t *testing.T) {
	var got tdTime
	err := json.Unmarshal([]byte(`"not a time"`), &got)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tdTime")
}
