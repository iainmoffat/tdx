package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestSearchPeople_FiltersClientsByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T","PrimaryEmail":"t@x"}`))
		case "/TDWebApi/api/people/lookup":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"UID":"u1","FullName":"Staff","IsEmployee":true,"IsActive":true},
				{"UID":"u2","FullName":"Client","IsEmployee":false,"IsActive":true}
			]`))
		default:
			t.Errorf("unexpected: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	res, _, err := searchPeopleHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, searchPeopleArgs{
		SearchText: "x",
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.peopleSearchResult", got["schema"])
	people, ok := got["people"].([]any)
	require.True(t, ok)
	require.Len(t, people, 1, "client filtered out by default")
}

func TestSearchPeople_IncludeClients(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/people/lookup":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"UID":"u1","FullName":"Staff","IsEmployee":true},
				{"UID":"u2","FullName":"Client","IsEmployee":false}
			]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	res, _, err := searchPeopleHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, searchPeopleArgs{
		SearchText:     "x",
		IncludeClients: true,
	})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	people, ok := got["people"].([]any)
	require.True(t, ok)
	require.Len(t, people, 2, "both staff and client present with includeClients=true")
}

func TestGetPerson_Shape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/people/abc-uid":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"abc-uid","FullName":"Iain Moffat","PrimaryEmail":"ipm@ufl.edu","IsEmployee":true,"IsActive":true,"DefaultAccountName":"ICT"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	res, _, err := getPersonHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, getPersonArgs{UID: "abc-uid"})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.person", got["schema"])
	person, ok := got["person"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Iain Moffat", person["fullName"])
}

func TestListAccounts_SortedByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/accounts/search":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"ID":866,"Name":"ICT","IsActive":true},
				{"ID":1,"Name":"BOT","IsActive":true}
			]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	res, _, err := listAccountsHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, listAccountsArgs{})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.accountList", got["schema"])
	accounts, ok := got["accounts"].([]any)
	require.True(t, ok)
	require.Len(t, accounts, 2)
	first, ok := accounts[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "BOT", first["name"], "BOT < ICT alphabetically")
}

func TestListResourcePools_Shape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"u","FullName":"T"}`))
		case "/TDWebApi/api/resourcepools/search":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":46,"Name":"LPS","IsActive":true,"RequiresApproval":true}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	res, _, err := listResourcePoolsHandler(svcs)(context.Background(), &sdkmcp.CallToolRequest{}, listResourcePoolsArgs{})
	require.NoError(t, err)
	require.False(t, res.IsError)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(extractText(t, res)), &got))
	require.Equal(t, "tdx.v1.resourcePoolList", got["schema"])
	pools, ok := got["pools"].([]any)
	require.True(t, ok)
	require.Len(t, pools, 1)
}
