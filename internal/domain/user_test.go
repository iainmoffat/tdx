package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUser_DisplayName(t *testing.T) {
	u := User{FullName: "Iain Moffat", Email: "ipm@ufl.edu"}
	require.Equal(t, "Iain Moffat", u.DisplayName())
}

func TestUser_DisplayName_FallsBackToEmail(t *testing.T) {
	u := User{Email: "ipm@ufl.edu"}
	require.Equal(t, "ipm@ufl.edu", u.DisplayName())
}

func TestUser_DisplayName_FallsBackToUID(t *testing.T) {
	u := User{UID: "abcd-1234"}
	require.Equal(t, "abcd-1234", u.DisplayName())
}

func TestUser_DisplayName_EmptyFallback(t *testing.T) {
	u := User{}
	require.Equal(t, "(unknown user)", u.DisplayName())
}

func TestUser_IsZero(t *testing.T) {
	require.True(t, User{}.IsZero())
	require.False(t, User{UID: "x"}.IsZero())
}

func TestUser_HasManagerFields(t *testing.T) {
	u := User{
		UID:            "user-1",
		FullName:       "Iain Moffat",
		Email:          "ipm@ufl.edu",
		Active:         true,
		AccountName:    "UFIT Operations",
		ReportsToUID:   "mgr-uid",
		ReportsToID:    42,
		ReportsToName:  "Manager Name",
		ReportsToEmail: "mgr@ufl.edu",
	}
	require.Equal(t, "user-1", u.UID)
	require.Equal(t, "mgr-uid", u.ReportsToUID)
	require.Equal(t, 42, u.ReportsToID)
	require.Equal(t, "Manager Name", u.ReportsToName)
	require.Equal(t, "mgr@ufl.edu", u.ReportsToEmail)
	require.True(t, u.Active)
	require.Equal(t, "UFIT Operations", u.AccountName)
}
