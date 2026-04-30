package peoplesvc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const poolsFixture = `[
	{"ID":2,"Name":"ADI - Business Analysis\t","IsActive":true,"RequiresApproval":true,"ManagerUID":"00000000-0000-0000-0000-000000000000","ManagerFullName":null},
	{"ID":46,"Name":"ICT - DBP - Linux Platform Services LPS\t","IsActive":true,"RequiresApproval":true,"ManagerUID":"d44687e1-1a09-ef11-86d4-df13b8e4e655","ManagerFullName":"Iain Moffat"},
	{"ID":74,"Name":"UFIT Leaders","IsActive":true,"RequiresApproval":false,"ManagerUID":"00000000-0000-0000-0000-000000000000","ManagerFullName":null}
]`

func newPoolsServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/TDWebApi/api/resourcepools/search", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		require.Contains(t, string(body), "{}")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(poolsFixture))
	}))
}

func TestSearchPools_DecodesAndTrimsNames(t *testing.T) {
	srv := newPoolsServer(t)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	pools, err := svc.SearchPools(context.Background(), profile)
	require.NoError(t, err)
	require.Len(t, pools, 3)
	require.Equal(t, "ADI - Business Analysis", pools[0].Name)
	require.Equal(t, "ICT - DBP - Linux Platform Services LPS", pools[1].Name)
	require.Equal(t, 46, pools[1].ID)
	require.Equal(t, "Iain Moffat", pools[1].ManagerFullName)
}

func TestResolvePoolByName_ExactMatch(t *testing.T) {
	srv := newPoolsServer(t)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	pool, err := svc.ResolvePoolByName(context.Background(), profile, "ICT - DBP - Linux Platform Services LPS")
	require.NoError(t, err)
	require.Equal(t, 46, pool.ID)
}

func TestResolvePoolByName_TrimsAndCaseInsensitive(t *testing.T) {
	srv := newPoolsServer(t)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	pool, err := svc.ResolvePoolByName(context.Background(), profile, "  ufit leaders  ")
	require.NoError(t, err)
	require.Equal(t, 74, pool.ID)
}

func TestResolvePoolByName_NotFound(t *testing.T) {
	srv := newPoolsServer(t)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.ResolvePoolByName(context.Background(), profile, "Does Not Exist")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestResolvePoolByName_EmptyName(t *testing.T) {
	srv := newPoolsServer(t)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.ResolvePoolByName(context.Background(), profile, "  ")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}

func TestResolvePoolByName_AmbiguousMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"ID":1,"Name":"Dup\t","IsActive":true},
			{"ID":2,"Name":"DUP","IsActive":true}
		]`))
	}))
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.ResolvePoolByName(context.Background(), profile, "dup")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ambiguous")
	require.Contains(t, err.Error(), "1")
	require.Contains(t, err.Error(), "2")
}
