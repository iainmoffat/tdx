package people

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/domain"
)

func searchFixture() []domain.User {
	return []domain.User{
		{UID: "11111111-aaaa", FullName: "Alice Staff", Email: "alice@x", IsEmployee: true, Active: true, AccountName: "ICT", ReportsToName: "Mgr A", Title: "Engineer"},
		{UID: "22222222-bbbb", FullName: "Bob Staff", Email: "bob@x", IsEmployee: true, Active: true},
		{UID: "33333333-cccc", FullName: "Charlie Client", Email: "charlie@x", IsEmployee: false, Active: true},
	}
}

func TestPeopleSearch_DefaultUsesSearchAndFiltersToStaff(t *testing.T) {
	stub := &stubPeoplesvc{searchUsers: searchFixture()}
	var out bytes.Buffer
	require.NoError(t, runPeopleSearch(context.Background(), &out, stub, "default", "Staff", 25, false, false))
	got := out.String()
	require.Contains(t, got, "Alice Staff")
	require.Contains(t, got, "Bob Staff")
	require.NotContains(t, got, "Charlie Client", "client never reaches the search-employees-only path")
	require.NotNil(t, stub.lastSearchFilter.Employee)
	require.True(t, *stub.lastSearchFilter.Employee, "default path must set IsEmployee=true server-side")
}

func TestPeopleSearch_IncludeClients_UsesLookup(t *testing.T) {
	stub := &stubPeoplesvc{lookupHits: searchFixture()}
	var out bytes.Buffer
	require.NoError(t, runPeopleSearch(context.Background(), &out, stub, "default", "Staff", 25, true, false))
	got := out.String()
	require.Contains(t, got, "Charlie Client", "client should appear with --include-clients")
	require.Equal(t, "Staff", stub.lastLookupQuery, "include-clients path uses lookup")
}

func TestPeopleSearch_NoStaffMatchesHintsIncludeClients(t *testing.T) {
	stub := &stubPeoplesvc{searchUsers: []domain.User{
		{UID: "11111111-aaaa", FullName: "Alice Staff", IsEmployee: true},
	}}
	var out bytes.Buffer
	require.NoError(t, runPeopleSearch(context.Background(), &out, stub, "default", "Charlie", 25, false, false))
	require.Contains(t, out.String(), "no staff match")
	require.Contains(t, out.String(), "--include-clients")
}

func TestPeopleSearch_IncludeClientsNoResults(t *testing.T) {
	stub := &stubPeoplesvc{lookupHits: nil}
	var out bytes.Buffer
	require.NoError(t, runPeopleSearch(context.Background(), &out, stub, "default", "nobody", 25, true, false))
	require.Contains(t, out.String(), "no people match")
	require.NotContains(t, out.String(), "--include-clients")
}

func TestPeopleSearch_PassesQueryAndLimit_LookupPath(t *testing.T) {
	stub := &stubPeoplesvc{}
	var out bytes.Buffer
	require.NoError(t, runPeopleSearch(context.Background(), &out, stub, "default", "Iain Mof", 7, true, false))
	require.Equal(t, "Iain Mof", stub.lastLookupQuery)
	require.Equal(t, 7, stub.lastLookupMax)
}

func TestPeopleSearch_StaffPathSubstringMatch(t *testing.T) {
	stub := &stubPeoplesvc{searchUsers: []domain.User{
		{UID: "u1", FullName: "Iain Moffat", Email: "ipm@ufl.edu", IsEmployee: true},
		{UID: "u2", FullName: "Aaron Lamotte", Email: "aaron@ufl.edu", IsEmployee: true},
	}}
	var out bytes.Buffer
	require.NoError(t, runPeopleSearch(context.Background(), &out, stub, "default", "iain", 25, false, false))
	got := out.String()
	require.Contains(t, got, "Iain Moffat")
	require.NotContains(t, got, "Aaron Lamotte")
}

func TestPeopleSearch_JSONEnvelope(t *testing.T) {
	stub := &stubPeoplesvc{searchUsers: searchFixture()}
	var out bytes.Buffer
	require.NoError(t, runPeopleSearch(context.Background(), &out, stub, "default", "Staff", 25, false, true))
	var got map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Equal(t, "tdx.v1.peopleSearchResult", got["schema"])
	require.Equal(t, "Staff", got["query"])
	people, ok := got["people"].([]any)
	require.True(t, ok)
	require.Len(t, people, 2, "two staff matches")
	first, ok := people[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "11111111-aaaa", first["uid"])
	require.Equal(t, "Alice Staff", first["fullName"])
	require.Equal(t, true, first["isEmployee"])
}

func TestPeopleSearch_PropagatesError(t *testing.T) {
	stub := &stubPeoplesvc{searchUsersErr: errors.New("boom")}
	var out bytes.Buffer
	err := runPeopleSearch(context.Background(), &out, stub, "default", "x", 25, false, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestPeopleSearch_TitleColumnAppears(t *testing.T) {
	stub := &stubPeoplesvc{searchUsers: searchFixture()}
	var out bytes.Buffer
	require.NoError(t, runPeopleSearch(context.Background(), &out, stub, "default", "Staff", 25, false, false))
	got := out.String()
	require.True(t, strings.Contains(got, "TITLE"))
	require.Contains(t, got, "Engineer", "Alice's title should render")
}

func TestSearchCmd_FlagsRegistered(t *testing.T) {
	cmd := newCmdWith(nil)
	searchCmd, _, err := cmd.Find([]string{"search"})
	require.NoError(t, err)
	for _, name := range []string{"limit", "include-clients", "json", "profile"} {
		require.NotNil(t, searchCmd.Flags().Lookup(name), "missing flag: %s", name)
	}
}
