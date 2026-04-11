package timesvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListTimeTypes_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/time/types", r.URL.Path)
		require.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"ID":1,"Name":"Development","Code":"DEV","HelpText":"Billable engineering work","IsBillable":true,"IsLimited":false,"IsActive":true},
			{"ID":17,"Name":"General Admin","IsBillable":false,"IsLimited":false,"IsActive":true},
			{"ID":42,"Name":"Meetings","IsBillable":false,"IsLimited":true,"IsActive":false}
		]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	types, err := svc.ListTimeTypes(context.Background(), profile)
	require.NoError(t, err)
	require.Len(t, types, 3)

	require.Equal(t, 1, types[0].ID)
	require.Equal(t, "Development", types[0].Name)
	require.Equal(t, "DEV", types[0].Code)
	require.True(t, types[0].Billable)
	require.False(t, types[0].Limited)
	require.True(t, types[0].Active)
	require.Equal(t, "Billable engineering work", types[0].Description)

	require.Equal(t, 42, types[2].ID)
	require.True(t, types[2].Limited)
	require.False(t, types[2].Active)
}

func TestListTimeTypes_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	_, err := svc.ListTimeTypes(context.Background(), profile)
	require.Error(t, err)
}
