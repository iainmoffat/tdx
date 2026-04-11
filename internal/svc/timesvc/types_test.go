package timesvc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ipm/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestTdTime_UnmarshalRFC3339Z(t *testing.T) {
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`"2026-04-06T00:00:00Z"`), &got))
	require.Equal(t, 2026, got.Time.Year())
	require.Equal(t, 4, int(got.Time.Month()))
	require.Equal(t, 6, got.Time.Day())
	require.Equal(t, time.UTC, got.Time.Location())
}

func TestTdTime_UnmarshalRFC3339Nano(t *testing.T) {
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`"2026-04-06T15:22:01.607Z"`), &got))
	require.Equal(t, 15, got.Time.Hour())
	require.Equal(t, 22, got.Time.Minute())
	require.Equal(t, 607000000, got.Time.Nanosecond())
	require.Equal(t, time.UTC, got.Time.Location())
}

func TestTdTime_UnmarshalNoZoneWithFractional(t *testing.T) {
	// This is the format that broke the Phase 2 walkthrough.
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`"2026-04-03T15:22:01.607"`), &got))
	require.Equal(t, 2026, got.Time.Year())
	require.Equal(t, 4, int(got.Time.Month()))
	require.Equal(t, 3, got.Time.Day())
	require.Equal(t, 15, got.Time.Hour())
	require.Equal(t, 22, got.Time.Minute())
	require.Equal(t, 1, got.Time.Second())
	// Wall-clock value with no zone is interpreted as EasternTZ.
	require.Equal(t, domain.EasternTZ, got.Time.Location())
}

func TestTdTime_UnmarshalNoZoneNoFractional(t *testing.T) {
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`"2026-04-03T15:22:01"`), &got))
	require.Equal(t, 15, got.Time.Hour())
	require.Equal(t, 22, got.Time.Minute())
	require.Equal(t, 1, got.Time.Second())
	require.Equal(t, domain.EasternTZ, got.Time.Location())
}

func TestTdTime_UnmarshalEmpty(t *testing.T) {
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`""`), &got))
	require.True(t, got.Time.IsZero())
}

func TestTdTime_UnmarshalNull(t *testing.T) {
	var got tdTime
	require.NoError(t, json.Unmarshal([]byte(`null`), &got))
	require.True(t, got.Time.IsZero())
}

func TestTdTime_UnmarshalGarbage(t *testing.T) {
	var got tdTime
	err := json.Unmarshal([]byte(`"not a time"`), &got)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tdTime")
}
