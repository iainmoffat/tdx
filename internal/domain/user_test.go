package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUser_DisplayName(t *testing.T) {
	u := User{FullName: "Sample User", Email: "sample@example.com"}
	require.Equal(t, "Sample User", u.DisplayName())
}

func TestUser_DisplayName_FallsBackToEmail(t *testing.T) {
	u := User{Email: "sample@example.com"}
	require.Equal(t, "sample@example.com", u.DisplayName())
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
		FullName:       "Sample User",
		Email:          "sample@example.com",
		Active:         true,
		AccountName:    "Sample Operations",
		ReportsToUID:   "mgr-uid",
		ReportsToID:    42,
		ReportsToName:  "Manager Name",
		ReportsToEmail: "mgr@example.com",
	}
	require.Equal(t, "user-1", u.UID)
	require.Equal(t, "mgr-uid", u.ReportsToUID)
	require.Equal(t, 42, u.ReportsToID)
	require.Equal(t, "Manager Name", u.ReportsToName)
	require.Equal(t, "mgr@example.com", u.ReportsToEmail)
	require.True(t, u.Active)
	require.Equal(t, "Sample Operations", u.AccountName)
}

func TestUserWorkableHoursZeroValue(t *testing.T) {
	var u User
	if u.WorkableHours != 0 {
		t.Errorf("zero value should be 0.0, got %v", u.WorkableHours)
	}
}

func TestUserWorkableHoursRoundTrip(t *testing.T) {
	u := User{UID: "u1", WorkableHours: 32.5}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var got User
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.WorkableHours != 32.5 {
		t.Errorf("round-trip lost value; got %v", got.WorkableHours)
	}
}
