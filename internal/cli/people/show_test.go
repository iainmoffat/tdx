package people

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/domain"
)

func showFixture() domain.User {
	return domain.User{
		UID: "aaaaaaaa-1234-5678-9abc-def012345678", FullName: "Sample User",
		Email: "sample@example.com", Active: true, IsEmployee: true,
		AccountName:      "999999 (Sample Department)",
		ReportsToName:    "John Toner",
		ReportsToEmail:   "john.toner@example.com",
		ResourcePoolName: "Sample Leaders",
		Title:            "Director",
	}
}

func TestPeopleShow_Text(t *testing.T) {
	stub := &stubPeoplesvc{users: map[string]domain.User{showFixture().UID: showFixture()}}
	var out bytes.Buffer
	require.NoError(t, runPeopleShow(context.Background(), &out, stub, "default", showFixture().UID, false))
	s := out.String()
	require.Contains(t, s, "UID:")
	require.Contains(t, s, "Sample User")
	require.Contains(t, s, "sample@example.com")
	require.Contains(t, s, "Active:        yes")
	require.Contains(t, s, "Employee:      yes")
	require.Contains(t, s, "Title:         Director")
	require.Contains(t, s, "Account:       999999")
	require.Contains(t, s, "Resource pool: Sample Leaders")
	require.Contains(t, s, "Manager:       John Toner <john.toner@example.com>")
}

func TestPeopleShow_JSON(t *testing.T) {
	stub := &stubPeoplesvc{users: map[string]domain.User{showFixture().UID: showFixture()}}
	var out bytes.Buffer
	require.NoError(t, runPeopleShow(context.Background(), &out, stub, "default", showFixture().UID, true))
	var got map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Equal(t, "tdx.v1.person", got["schema"])
	person, ok := got["person"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Sample User", person["fullName"])
}

func TestPeopleShow_NotFound(t *testing.T) {
	stub := &stubPeoplesvc{getErr: errors.New("404")}
	var out bytes.Buffer
	err := runPeopleShow(context.Background(), &out, stub, "default", "no-such-uid", false)
	require.Error(t, err)
}
