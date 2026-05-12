package projectsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListResources_DecodesThreeResources(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/TDWebApi/api/projects/259/resources", r.URL.Path)
		require.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"UID":"aaa-111","FullName":"Alice Smith","RoleID":5,"RoleName":"Developer","IsActive":true},
			{"UID":"bbb-222","FullName":"Bob Jones"},
			{"UID":"ccc-333","FullName":"Carol Brown","IsActive":false}
		]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	resources, err := svc.ListResources(context.Background(), profile, 259)
	require.NoError(t, err)
	require.Len(t, resources, 3)

	// First: all fields populated
	r0 := resources[0]
	require.Equal(t, "aaa-111", r0.UID)
	require.Equal(t, "Alice Smith", r0.FullName)
	require.Equal(t, 5, r0.RoleID)
	require.Equal(t, "Developer", r0.RoleName)
	require.True(t, r0.IsActive)

	// Second: only UID and FullName — optional fields should be zero
	r1 := resources[1]
	require.Equal(t, "bbb-222", r1.UID)
	require.Equal(t, "Bob Jones", r1.FullName)
	require.Equal(t, 0, r1.RoleID)
	require.Equal(t, "", r1.RoleName)
	require.False(t, r1.IsActive)

	// Third: inactive
	r2 := resources[2]
	require.Equal(t, "ccc-333", r2.UID)
	require.False(t, r2.IsActive)
}

func TestListResources_EmptyArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	resources, err := svc.ListResources(context.Background(), profile, 1)
	require.NoError(t, err)
	require.Empty(t, resources)
}

func TestListResources_URLContainsProjectID(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	svc, profile := harness(t, srv.URL)
	_, err := svc.ListResources(context.Background(), profile, 42)
	require.NoError(t, err)
	require.Equal(t, "/TDWebApi/api/projects/42/resources", gotPath)
}
