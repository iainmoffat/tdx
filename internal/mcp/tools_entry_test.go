package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListTimeEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/TDWebApi/api/time/search" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"TimeID": 100,
					"ItemID": 12345,
					"ItemTitle": "Fix login bug",
					"AppID": 42,
					"AppName": "IT Help Desk",
					"Component": 9,
					"TicketID": 12345,
					"TimeDate": "2026-04-06T00:00:00Z",
					"Minutes": 60,
					"Description": "Debugging",
					"TimeTypeID": 1,
					"TimeTypeName": "Development",
					"Billable": false,
					"Uid": "uid-abc",
					"Status": 0,
					"CreatedDate": "2026-04-06T10:00:00Z",
					"ModifiedDate": "2026-04-06T10:00:00Z"
				}
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := listEntriesHandler(svcs)
	result, _, err := handler(context.Background(), nil, listEntriesArgs{
		From: "2026-04-05",
		To:   "2026-04-11",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := extractText(t, result)
	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &entries))
	require.Len(t, entries, 1)
	require.Equal(t, float64(100), entries[0]["id"])
}

func TestGetTimeEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/TDWebApi/api/time/999" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID": 999,
				"ItemID": 12345,
				"ItemTitle": "Fix login bug",
				"AppID": 42,
				"AppName": "IT Help Desk",
				"Component": 9,
				"TicketID": 12345,
				"TimeDate": "2026-04-06T00:00:00Z",
				"Minutes": 120,
				"Description": "Fixing the bug",
				"TimeTypeID": 1,
				"TimeTypeName": "Development",
				"Billable": false,
				"Uid": "uid-abc",
				"Status": 0,
				"CreatedDate": "2026-04-06T10:00:00Z",
				"ModifiedDate": "2026-04-06T10:00:00Z"
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := getEntryHandler(svcs)
	result, _, err := handler(context.Background(), nil, getEntryArgs{ID: 999})
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := extractText(t, result)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &entry))
	require.Equal(t, float64(999), entry["id"])
}

func TestGetTimeEntry_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`"Not found"`))
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := getEntryHandler(svcs)
	result, _, err := handler(context.Background(), nil, getEntryArgs{ID: 999})
	require.NoError(t, err)
	require.True(t, result.IsError)
}
