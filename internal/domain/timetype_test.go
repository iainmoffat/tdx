package domain

import (
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
