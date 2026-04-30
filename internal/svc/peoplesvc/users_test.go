package peoplesvc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGetUser_Decodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/people/target-uid", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"UID": "target-uid",
			"ID":  100,
			"FullName": "Iain Moffat",
			"PrimaryEmail": "ipm@ufl.edu",
			"IsActive": true,
			"DefaultAccountName": "UFIT",
			"ReportsToUid": "mgr-uid",
			"ReportsToId":  42,
			"ReportsToFullName": "Manager Name",
			"ReportsToEmail":   "mgr@ufl.edu"
		}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	u, err := svc.GetUser(context.Background(), profile, "target-uid")
	require.NoError(t, err)
	require.Equal(t, domain.User{
		ID:             100,
		UID:            "target-uid",
		FullName:       "Iain Moffat",
		Email:          "ipm@ufl.edu",
		Active:         true,
		AccountName:    "UFIT",
		ReportsToUID:   "mgr-uid",
		ReportsToID:    42,
		ReportsToName:  "Manager Name",
		ReportsToEmail: "mgr@ufl.edu",
	}, u)
}

func TestGetUser_FallsBackToAlternateEmail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"UID": "target-uid",
			"FullName": "Test",
			"AlternateEmail": "alt@example.com"
		}`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	u, err := svc.GetUser(context.Background(), profile, "target-uid")
	require.NoError(t, err)
	require.Equal(t, "alt@example.com", u.Email)
}

func TestSearchUsers_DefaultsApplied(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/TDWebApi/api/people/search", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"UID":"u1","FullName":"User One","PrimaryEmail":"u1@x.com","IsActive":true}]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	users, err := svc.SearchUsers(context.Background(), profile, domain.UserFilter{})
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, "u1", users[0].UID)
	body := string(receivedBody)
	require.Contains(t, body, `"UserType":"User"`)
	require.Contains(t, body, `"IsActive":true`)
	require.Contains(t, body, `"MaxResults":100`)
	require.NotContains(t, body, `"IsEmployee"`)
}

func TestSearchUsers_EmployeeFilterSetsWireField(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	tt := true
	_, err := svc.SearchUsers(context.Background(), profile, domain.UserFilter{
		Employee: &tt,
		Limit:    5000,
	})
	require.NoError(t, err)
	body := string(receivedBody)
	require.Contains(t, body, `"IsEmployee":true`)
	require.Contains(t, body, `"MaxResults":5000`)
}

func TestSearchUsers_DecodesResourcePool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{
			"UID": "u1",
			"FullName": "User One",
			"PrimaryEmail": "u1@x.com",
			"IsActive": true,
			"ResourcePoolID": 46,
			"ResourcePoolName": "ICT - DBP - Linux Platform Services LPS\t"
		}]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	users, err := svc.SearchUsers(context.Background(), profile, domain.UserFilter{})
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, 46, users[0].ResourcePoolID)
	require.Equal(t, "ICT - DBP - Linux Platform Services LPS", users[0].ResourcePoolName)
}
