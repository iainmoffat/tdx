package peoplesvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const lookupFixture = `[
	{"UID":"aaaaaaaa-1234-5678-9abc-def012345678","FullName":"Sample User","PrimaryEmail":"sample@example.com","IsActive":true,"IsEmployee":true,"DefaultAccountName":"999999 (Sample Department)","Title":"Director"},
	{"UID":"abc12345-aaaa-bbbb-cccc-ddddeeeeffff","FullName":"Test Client","PrimaryEmail":"client@example.com","IsActive":true,"IsEmployee":false}
]`

func TestLookupPeople_QueryParamsAndDecode(t *testing.T) {
	var lastQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/TDWebApi/api/people/lookup", r.URL.Path)
		lastQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(lookupFixture))
	}))
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	users, err := svc.LookupPeople(context.Background(), profile, "Iain", 5)
	require.NoError(t, err)
	require.Len(t, users, 2)

	require.Contains(t, lastQuery, "searchText=Iain")
	require.Contains(t, lastQuery, "maxResults=5")

	require.Equal(t, "Sample User", users[0].FullName)
	require.True(t, users[0].IsEmployee)
	require.Equal(t, "Director", users[0].Title)
	require.False(t, users[1].IsEmployee, "client should decode as IsEmployee=false")
}

func TestLookupPeople_DefaultMaxResults(t *testing.T) {
	var lastQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.LookupPeople(context.Background(), profile, "x", 0)
	require.NoError(t, err)
	require.Contains(t, lastQuery, "maxResults=25")
}

func TestLookupPeople_CapsMaxResults(t *testing.T) {
	var lastQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.LookupPeople(context.Background(), profile, "x", 9999)
	require.NoError(t, err)
	require.Contains(t, lastQuery, "maxResults=100")
}

func TestLookupPeople_URLEncodesSearchText(t *testing.T) {
	var lastQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.LookupPeople(context.Background(), profile, "Sample User", 10)
	require.NoError(t, err)
	require.True(t,
		strings.Contains(lastQuery, "searchText=Sample+User") ||
			strings.Contains(lastQuery, "searchText=Sample%20User"),
		"expected URL-encoded space, got: %s", lastQuery)
}
