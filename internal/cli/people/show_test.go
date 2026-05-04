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
		UID: "d44687e1-1a09-ef11-86d4-df13b8e4e655", FullName: "Iain Moffat",
		Email: "ipm@ufl.edu", Active: true, IsEmployee: true,
		AccountName:      "14300000 (IT-ICT INFRA COMM TECHNOLOGY)",
		ReportsToName:    "John Toner",
		ReportsToEmail:   "john.toner@ufl.edu",
		ResourcePoolName: "UFIT Leaders",
		Title:            "Director",
	}
}

func TestPeopleShow_Text(t *testing.T) {
	stub := &stubPeoplesvc{users: map[string]domain.User{showFixture().UID: showFixture()}}
	var out bytes.Buffer
	require.NoError(t, runPeopleShow(context.Background(), &out, stub, "default", showFixture().UID, false))
	s := out.String()
	require.Contains(t, s, "UID:")
	require.Contains(t, s, "Iain Moffat")
	require.Contains(t, s, "ipm@ufl.edu")
	require.Contains(t, s, "Active:        yes")
	require.Contains(t, s, "Employee:      yes")
	require.Contains(t, s, "Title:         Director")
	require.Contains(t, s, "Account:       14300000")
	require.Contains(t, s, "Resource pool: UFIT Leaders")
	require.Contains(t, s, "Manager:       John Toner <john.toner@ufl.edu>")
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
	require.Equal(t, "Iain Moffat", person["fullName"])
}

func TestPeopleShow_NotFound(t *testing.T) {
	stub := &stubPeoplesvc{getErr: errors.New("404")}
	var out bytes.Buffer
	err := runPeopleShow(context.Background(), &out, stub, "default", "no-such-uid", false)
	require.Error(t, err)
}
