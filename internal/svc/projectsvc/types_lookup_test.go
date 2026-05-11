package projectsvc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListProjectTypes_ActiveOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "isActive=true") {
			t.Errorf("expected isActive=true query param, got: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[
			{"ID": 1, "Name": "IT Project", "IsActive": true},
			{"ID": 2, "Name": "Regulatory Project", "IsActive": true}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	types, err := svc.ListProjectTypes(context.Background(), prof, false)
	require.NoError(t, err)
	require.Len(t, types, 2)
	require.Equal(t, "IT Project", types[0].Name)
}

func TestListProjectTypes_IncludeInactive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should NOT include isActive param
		if strings.Contains(r.URL.RawQuery, "isActive") {
			t.Errorf("should not have isActive param when includeInactive=true, got: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.ListProjectTypes(context.Background(), prof, true)
	require.NoError(t, err)
}

func TestResolveTypeByName_Match(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"ID": 1, "Name": "IT Project", "IsActive": true},
			{"ID": 2, "Name": "Regulatory Project", "IsActive": true}
		]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	pt, err := svc.ResolveTypeByName(context.Background(), prof, "IT Project")
	require.NoError(t, err)
	require.Equal(t, 1, pt.ID)
}

func TestResolveTypeByName_CaseInsensitive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"ID": 1, "Name": "IT Project", "IsActive": true}]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	pt, err := svc.ResolveTypeByName(context.Background(), prof, "it project")
	require.NoError(t, err)
	require.Equal(t, 1, pt.ID)
}

func TestResolveTypeByName_NoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"ID": 1, "Name": "IT Project", "IsActive": true}]`))
	}))
	defer srv.Close()
	svc, prof := harness(t, srv.URL)
	_, err := svc.ResolveTypeByName(context.Background(), prof, "Nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no project type matches")
}
