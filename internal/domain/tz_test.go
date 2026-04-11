package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEasternTZ_IsLoaded(t *testing.T) {
	require.NotNil(t, EasternTZ)
	require.Equal(t, "America/New_York", EasternTZ.String())
}

func TestEasternTZ_ConvertsUTCCorrectly(t *testing.T) {
	// 2026-07-04 12:00 UTC is 08:00 EDT
	utc := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	eastern := utc.In(EasternTZ)
	require.Equal(t, 8, eastern.Hour())

	// 2026-01-15 12:00 UTC is 07:00 EST
	utcWinter := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	easternWinter := utcWinter.In(EasternTZ)
	require.Equal(t, 7, easternWinter.Hour())
}
