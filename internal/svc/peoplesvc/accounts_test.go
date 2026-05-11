package peoplesvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const accountsFixture = `[
	{"ID":866,"Name":"999999 (Sample Department)","IsActive":true,"Code":"999999","ManagerUID":"aaaaaaaa-1234-5678-9abc-def012345678","ManagerFullName":"Sample User"},
	{"ID":1,"Name":"00000000 (BOARD OF TRUSTEES)","IsActive":true,"Code":"00000000","ManagerUID":"00000000-0000-0000-0000-000000000000","ManagerFullName":null},
	{"ID":3,"Name":"01000000 (OFFICE OF PRESIDENT)","IsActive":true,"Code":"01000000","ManagerUID":"00000000-0000-0000-0000-000000000000","ManagerFullName":null}
]`

func newAccountsServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/TDWebApi/api/accounts/search", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(accountsFixture))
	}))
}

func TestSearchAccounts_Decode(t *testing.T) {
	srv := newAccountsServer(t)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	accounts, err := svc.SearchAccounts(context.Background(), profile)
	require.NoError(t, err)
	require.Len(t, accounts, 3)
	require.Equal(t, 866, accounts[0].ID)
	require.Equal(t, "999999 (Sample Department)", accounts[0].Name)
	require.Equal(t, "Sample User", accounts[0].ManagerFullName)
	require.Equal(t, "999999", accounts[0].Code)
}

func TestResolveAccountByName_ExactMatch(t *testing.T) {
	srv := newAccountsServer(t)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	acct, err := svc.ResolveAccountByName(context.Background(), profile, "999999 (Sample Department)")
	require.NoError(t, err)
	require.Equal(t, 866, acct.ID)
}

func TestResolveAccountByName_CaseInsensitiveAndTrim(t *testing.T) {
	srv := newAccountsServer(t)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	acct, err := svc.ResolveAccountByName(context.Background(), profile, "  00000000 (board of trustees)  ")
	require.NoError(t, err)
	require.Equal(t, 1, acct.ID)
}

func TestResolveAccountByName_NotFound(t *testing.T) {
	srv := newAccountsServer(t)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.ResolveAccountByName(context.Background(), profile, "nope")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
	require.Contains(t, err.Error(), "3 accounts")
}

func TestResolveAccountByName_EmptyName(t *testing.T) {
	srv := newAccountsServer(t)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.ResolveAccountByName(context.Background(), profile, "  ")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}

func TestResolveAccountByName_Ambiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"ID":10,"Name":"Same Name","IsActive":true},
			{"ID":11,"Name":"SAME NAME","IsActive":true}
		]`))
	}))
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.ResolveAccountByName(context.Background(), profile, "Same Name")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ambiguous")
	require.Contains(t, err.Error(), "10")
	require.Contains(t, err.Error(), "11")
}
