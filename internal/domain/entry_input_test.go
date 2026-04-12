package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEntryUpdate_IsEmpty(t *testing.T) {
	u := EntryUpdate{}
	require.True(t, u.IsEmpty())

	desc := "hello"
	u.Description = &desc
	require.False(t, u.IsEmpty())
}

func TestEntryUpdate_IsEmpty_AllFields(t *testing.T) {
	mins := 30
	typeID := 5
	billable := true
	desc := "test"
	u := EntryUpdate{
		Minutes:     &mins,
		TimeTypeID:  &typeID,
		Billable:    &billable,
		Description: &desc,
	}
	require.False(t, u.IsEmpty())
}
