package timesvc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/domain"
)

// timeOffSearchServer returns a stub TD server whose /time/search responds with
// the supplied JSON array body.
func timeOffSearchServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/TDWebApi/api/time/types" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if r.URL.Path == "/TDWebApi/api/time/search" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestResolveTimeOffItemID_OverrideWins(t *testing.T) {
	// The server must never be called when an override is supplied.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("override path must not call the API (got %s)", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	got, err := svc.ResolveTimeOffItemID(context.Background(), profile, "uid-abc", 52)
	require.NoError(t, err)
	require.Equal(t, 52, got)
}

func TestResolveTimeOffItemID_DiscoversMostRecent(t *testing.T) {
	// Component 17 = TimeOff. The 2026-05-14 entry is newer, so its ProjectID
	// (which decodes into Target.ItemID) must win over the older 2026-05-12 one.
	srv := timeOffSearchServer(t, `[
		{"TimeID":1,"Component":17,"ProjectID":40,"TimeDate":"2026-05-12T00:00:00Z","Minutes":60,"TimeTypeID":3,"Uid":"uid-abc","Status":0},
		{"TimeID":2,"Component":17,"ProjectID":52,"TimeDate":"2026-05-14T00:00:00Z","Minutes":60,"TimeTypeID":3,"Uid":"uid-abc","Status":0},
		{"TimeID":3,"Component":9,"TicketID":900,"ItemID":900,"TimeDate":"2026-05-20T00:00:00Z","Minutes":60,"TimeTypeID":1,"Uid":"uid-abc","Status":0}
	]`)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	got, err := svc.ResolveTimeOffItemID(context.Background(), profile, "uid-abc", 0)
	require.NoError(t, err)
	require.Equal(t, 52, got)
}

func TestResolveTimeOffItemID_NotFound(t *testing.T) {
	// Only non-time-off entries: discovery must report the sentinel.
	srv := timeOffSearchServer(t, `[
		{"TimeID":3,"Component":9,"TicketID":900,"ItemID":900,"TimeDate":"2026-05-20T00:00:00Z","Minutes":60,"TimeTypeID":1,"Uid":"uid-abc","Status":0}
	]`)
	defer srv.Close()
	svc, profile := harness(t, srv.URL)

	_, err := svc.ResolveTimeOffItemID(context.Background(), profile, "uid-abc", 0)
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrTimeOffIDUnknown),
		"error %v should wrap domain.ErrTimeOffIDUnknown", err)
}
