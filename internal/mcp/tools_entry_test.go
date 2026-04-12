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

func TestCreateEntry_WithConfirm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/TDWebApi/api/auth/getuser":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"UID":"uid-abc","FullName":"Test User","PrimaryEmail":"test@ufl.edu","ReferenceID":1}`))

		case r.Method == http.MethodPost && r.URL.Path == "/TDWebApi/api/time":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Succeeded":[{"Index":0,"ID":999}],"Failed":[]}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":12345,"ItemTitle":"Fix bug",
				"Uid":"uid-abc","TimeTypeID":1,"TimeTypeName":"","Billable":false,
				"AppID":42,"AppName":"IT Help Desk","Component":9,
				"TicketID":12345,"ProjectID":0,
				"TimeDate":"2026-04-07T00:00:00Z",
				"Minutes":60.0,"Description":"debugging",
				"Status":0,"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":1,"Name":"Development","IsActive":true}]`))

		default:
			t.Logf("unhandled: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := createEntryHandler(svcs)
	result, _, err := handler(context.Background(), nil, createEntryArgs{
		Date:        "2026-04-07",
		Hours:       1.0,
		TypeID:      1,
		Kind:        "ticket",
		ItemID:      12345,
		AppID:       42,
		Description: "debugging",
		Confirm:     true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected success, got: %v", extractText(t, result))

	text := extractText(t, result)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &entry))
	require.Equal(t, float64(999), entry["id"])
}

func TestCreateEntry_WithoutConfirm(t *testing.T) {
	svcs := mcpHarness(t, "http://localhost/")
	handler := createEntryHandler(svcs)
	result, _, err := handler(context.Background(), nil, createEntryArgs{
		Date:    "2026-04-07",
		Hours:   1.0,
		TypeID:  1,
		Kind:    "ticket",
		ItemID:  12345,
		Confirm: false,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	text := extractText(t, result)
	require.Contains(t, text, "confirm")
}

func TestUpdateEntry_WithConfirm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":12345,"ItemTitle":"Fix bug",
				"Uid":"uid-abc","TimeTypeID":1,"TimeTypeName":"Development","Billable":false,
				"AppID":42,"AppName":"IT Help Desk","Component":9,
				"TicketID":12345,"ProjectID":0,
				"TimeDate":"2026-04-07T00:00:00Z",
				"Minutes":60.0,"Description":"debugging",
				"Status":0,"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodPut && r.URL.Path == "/TDWebApi/api/time/999":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"TimeID":999,"ItemID":12345,"ItemTitle":"Fix bug",
				"Uid":"uid-abc","TimeTypeID":1,"TimeTypeName":"Development","Billable":false,
				"AppID":42,"AppName":"IT Help Desk","Component":9,
				"TicketID":12345,"ProjectID":0,
				"TimeDate":"2026-04-07T00:00:00Z",
				"Minutes":120.0,"Description":"debugging more",
				"Status":0,"CreatedDate":"0001-01-01T00:00:00","ModifiedDate":"0001-01-01T00:00:00"
			}`))

		case r.Method == http.MethodGet && r.URL.Path == "/TDWebApi/api/time/types":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"ID":1,"Name":"Development","IsActive":true}]`))

		default:
			t.Logf("unhandled: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := updateEntryHandler(svcs)
	result, _, err := handler(context.Background(), nil, updateEntryArgs{
		ID:          999,
		Hours:       2.0,
		Description: "debugging more",
		Confirm:     true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected success, got: %v", extractText(t, result))

	text := extractText(t, result)
	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &entry))
	require.Equal(t, float64(999), entry["id"])
}

func TestDeleteEntry_WithConfirm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && r.URL.Path == "/TDWebApi/api/time/999" {
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Logf("unhandled: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	svcs := mcpHarness(t, srv.URL)
	handler := deleteEntryHandler(svcs)
	result, _, err := handler(context.Background(), nil, deleteEntryArgs{
		ID:      999,
		Confirm: true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected success, got: %v", extractText(t, result))

	text := extractText(t, result)
	require.Contains(t, text, "999")
	require.Contains(t, text, "deleted")
}
