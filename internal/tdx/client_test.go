package tdx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClient_AttachesBearerToken(t *testing.T) {
	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "token-xyz")
	require.NoError(t, err)

	_, err = c.Do(context.Background(), http.MethodGet, "/api/ping", nil)
	require.NoError(t, err)
	require.Equal(t, "Bearer token-xyz", seenAuth)
}

func TestClient_ReturnsBodyOn2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)

	body, err := c.Do(context.Background(), http.MethodGet, "/api/thing", nil)
	require.NoError(t, err)
	require.Contains(t, string(body), `"ok":true`)
}

func TestClient_401ReturnsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"Message":"bad token"}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)

	_, err = c.Do(context.Background(), http.MethodGet, "/api/thing", nil)
	require.ErrorIs(t, err, ErrUnauthorized)
}

func TestClient_4xxReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`bad input`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)

	_, err = c.Do(context.Background(), http.MethodGet, "/api/thing", nil)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.Status)
	require.Contains(t, apiErr.Message, "bad input")
}

func TestClient_RetryAfterOn429(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)
	c.maxRetries = 2
	c.retryAfterCap = time.Millisecond

	_, err = c.Do(context.Background(), http.MethodGet, "/api/thing", nil)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
}

func TestClient_RejectsInvalidBaseURL(t *testing.T) {
	_, err := NewClient("not a url", "t")
	require.Error(t, err)
}

func TestClient_PingCallsTimeTypes(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)

	require.NoError(t, c.Ping(context.Background()))
	require.Equal(t, "/TDWebApi/api/time/types", seenPath)
}

func TestClient_PingOnUnauthorizedReturnsErrInvalidToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "t")
	require.NoError(t, err)

	err = c.Ping(context.Background())
	require.ErrorIs(t, err, ErrUnauthorized)
}
