package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTimeType_HasLimit(t *testing.T) {
	require.False(t, TimeType{Limited: false}.HasLimit())
	require.True(t, TimeType{Limited: true}.HasLimit())
}

func TestTimeType_FindByID(t *testing.T) {
	types := []TimeType{
		{ID: 1, Name: "Development"},
		{ID: 17, Name: "General Admin"},
		{ID: 42, Name: "Meetings"},
	}
	got, ok := FindTimeTypeByID(types, 17)
	require.True(t, ok)
	require.Equal(t, "General Admin", got.Name)

	_, ok = FindTimeTypeByID(types, 999)
	require.False(t, ok)
}

func TestTimeType_FindByNameCaseInsensitive(t *testing.T) {
	types := []TimeType{
		{ID: 1, Name: "Development"},
		{ID: 17, Name: "General Admin"},
	}
	got, ok := FindTimeTypeByName(types, "development")
	require.True(t, ok)
	require.Equal(t, 1, got.ID)

	got, ok = FindTimeTypeByName(types, "GENERAL ADMIN")
	require.True(t, ok)
	require.Equal(t, 17, got.ID)

	_, ok = FindTimeTypeByName(types, "missing")
	require.False(t, ok)
}

func TestDefaultTimeOffType_SingleMatch(t *testing.T) {
	types := []TimeType{
		{ID: 1, Name: "Standard Activities", Active: true},
		{ID: 3, Name: "Leave", Active: true, IsTimeOff: true},
	}
	got, err := DefaultTimeOffType(types)
	require.NoError(t, err)
	require.Equal(t, 3, got.ID)
}

func TestDefaultTimeOffType_NoMatch(t *testing.T) {
	types := []TimeType{{ID: 1, Name: "Standard Activities", Active: true}}
	_, err := DefaultTimeOffType(types)
	require.Error(t, err)
}

func TestDefaultTimeOffType_MultipleMatches(t *testing.T) {
	types := []TimeType{
		{ID: 3, Name: "Leave", Active: true, IsTimeOff: true},
		{ID: 4, Name: "Holiday", Active: true, IsTimeOff: true},
	}
	_, err := DefaultTimeOffType(types)
	require.Error(t, err)
	// The error must name the candidates so the user knows what to pass to --type.
	msg := err.Error()
	require.True(t, strings.Contains(msg, "Leave") && strings.Contains(msg, "Holiday"),
		"error %q should name both candidates", msg)
}

func TestDefaultTimeOffType_IgnoresInactive(t *testing.T) {
	types := []TimeType{
		{ID: 3, Name: "Leave", Active: true, IsTimeOff: true},
		{ID: 9, Name: "Old Leave", Active: false, IsTimeOff: true},
	}
	got, err := DefaultTimeOffType(types)
	require.NoError(t, err)
	require.Equal(t, 3, got.ID, "inactive type must be ignored")
}
